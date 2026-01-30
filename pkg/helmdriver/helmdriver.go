// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package helmdriver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"datadog.com/helm-plugin/pkg/annotator"
	"datadog.com/helm-plugin/pkg/git"
	"go.yaml.in/yaml/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// Relying on this label to get the release name might be unreliable
	releaseNameLabel = "app.kubernetes.io/instance"
)

var _ Driver = (*driver)(nil)

type Driver interface {
	Execute() error
}

type driver struct {
	// args contains unpacked Helm arguments
	args []string
	in   io.Reader
	out  io.Writer
	// loc contains annotation body
	loc *annotator.LocationObject
	// chartURLOverride, if not empty, contains a value that will override detected chartURL.
	chartURLOverride string
	// pluginVersion contains the version of the plugin from plugin.yaml
	pluginVersion string
}

// New returns helm driver that runs annotator on in payload.
// Output is writen to the out writer.
// osargs contain only helm args separated by ;
// chartURLOverride may contain chartURL to use over detected one.
// pluginVersion may contain helm plugin version that attached verbatim.
func New(osargs []string, in io.Reader, out io.Writer, chartURLOverride, pluginVersion string) Driver {
	// Plugin accepts HELM args separated by ; in order to figure out chart location
	args := strings.Split(osargs[0], ";")

	d := &driver{
		// omit helm command
		args: args[1:],
		in:   in,
		out:  out,
		loc: &annotator.LocationObject{
			Helm: &annotator.HelmLocation{},
		},
		chartURLOverride: chartURLOverride,
		pluginVersion:    pluginVersion,
	}

	return d
}

func (d *driver) Execute() error {
	slog.Debug("Execute", "args", d.args)

	var input bytes.Buffer
	// Read all stdin into a buffer
	if _, err := io.Copy(&input, d.in); err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		return fmt.Errorf("io copy: %w", err)
	}

	// Make a copy for argparse
	incopy := input
	if err := d.parseArgs(&incopy); err != nil {
		return fmt.Errorf("parse args: %w", err)
	}

	if err := d.Run(&input); err != nil {
		fmt.Fprintf(os.Stderr, "post-render error: %v\n", err)
		return fmt.Errorf("run: %w", err)
	}

	return nil
}

// Run takes rendered manifests and returns modified manifests with added annotations.
func (d *driver) Run(in *bytes.Buffer) error {
	// Create config for the annotator
	cfg := &annotator.Config{
		Input:         in,
		Output:        d.out,
		YAMLIndent:    4,
		PluginVersion: d.pluginVersion,
	}

	// Create annotator with our config and location object
	ann, err := annotator.NewWithLocationObject(cfg, *d.loc)
	if err != nil {
		return fmt.Errorf("create annotator: %w", err)
	}

	if err := ann.Run(); err != nil {
		return fmt.Errorf("annotator run: %w", err)
	}

	return nil
}

// parseArgs is a custom helm argument parser use to capture chart path
// and values files. It is custom to avoid copying every single argument Helm supports.
func (d *driver) parseArgs(r io.Reader) error {
	releaseName := ""
	decoder := yaml.NewDecoder(r)
	for {
		var doc map[string]any
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("decode error: %w", err)
		}

		// Skip completely empty docs (e.g., trailing ---)
		if doc == nil {
			continue
		}

		u := &unstructured.Unstructured{Object: doc}
		labels := u.GetLabels()

		releaseName = labels[releaseNameLabel]
		if releaseName == "" {
			// Pick next doc to check it for release name
			continue
		}

		slog.Debug("Release name found", "name", releaseName)
		break
	}

	if releaseName == "" {
		return fmt.Errorf("empty release name")
	}

	// Now that release name is known can firugre out chart path from cmdline
	// as they go one after another expect with OCI is in use for install command.

	// Look for:
	// 1. --repo
	// 2. -f, --values
	// Throw out the rest
	var values []string
	var repo string
	var chartPath string

	for len(d.args) > 0 {
		arg := d.args[0]
		d.args = shift(d.args) // select next

		switch {
		case arg == "--repo":
			repo = d.args[0]
			d.args = shift(d.args) // select next

		case arg == "-f" || arg == "--values":
			values = append(values, d.args[0])
			d.args = shift(d.args) // select next

		case arg == releaseName:
			chartPath = d.args[0]
			if chartPath == "--version" {
				d.args = shift(d.args) // skip --version
				d.args = shift(d.args) // skip <x.y.z>
				// oci chart paths
				chartPath = d.args[0]
			}
			d.args = shift(d.args) // select next
		}
	}

	if chartPath == "" {
		return fmt.Errorf("unable to find chart name for given release name: %s", releaseName)
	}

	slog.Debug("Args:", "values", values, "repo", repo, "releaseName", releaseName, "chartPath", chartPath)

	// determine what chartPath contains
	// local path or a file or a link
	// 1. By chart reference: helm install mymaria example/mariadb
	// Assume chart is in local repo
	// Assume values is in local repo
	// 2. By path to a packaged chart: helm install mynginx ./nginx-1.2.3.tgz
	// Assume chart is in local repo
	// Assume values is in local repo
	// 3. By path to an unpacked chart directory: helm install mynginx ./nginx
	// ASsume char is in local repo
	// Assume values is in local repo
	// 4. By absolute URL: helm install mynginx https://example.com/charts/nginx-1.2.3.tgz
	// Assume values is in local
	// 5. By chart reference and repo url: helm install --repo https://example.com/charts/ mynginx nginx
	// Assume chart is in <repo> under ngins
	// Assume values is in local repo
	// 6. By OCI registries: helm install mynginx --version 1.2.3 oci://example.com/charts/nginx
	// Assume chart is in URL under
	// Assume values is in local repo

	dir := dirFromChart(chartPath)
	workTree, err := git.IsInsideRepo(dir)
	if err != nil {
		return fmt.Errorf("git.IsInsideRepo: %w", err)
	}

	if workTree {
		url, sha, err := git.GetRepoURLAndSHA(dir)
		if err != nil {
			return fmt.Errorf("git.GetRepoURLAndSHA: %w", err)
		}
		slog.Debug("Inside work tree", "url", url, "sha", sha)

		d.loc.Helm.ChartURL = chartURL(repo, chartPath, d.chartURLOverride)
		d.loc.Helm.RepoURL = url
		d.loc.Helm.TargetRevision = sha
		d.loc.Helm.ValuesPath = values
		d.loc.Helm.ChartPath = localChartPath(repo, chartPath)

		return nil
	}

	// If local location is not a git repo and override is set fill in what we have
	// This is suitable for local testing w/o git repo.
	if d.chartURLOverride != "" {
		d.loc.Helm.ChartURL = chartURL(repo, chartPath, d.chartURLOverride)
		d.loc.Helm.ValuesPath = values
		d.loc.Helm.ChartPath = localChartPath(repo, chartPath)

		return nil
	}

	// Current directory is not a git repo
	// User need to provide Chart repo using DD_HELM_CHART_URL
	return fmt.Errorf("unable to determine chart repo, provide it manualy using DD_HELM_CHART_URL")
}

// dirFromChart gets a path to possible git dir where to run git command
func dirFromChart(chartPath string) string {
	// if local dir, use dir
	// if file, use dirname(of file)
	// use cwd
	stat, err := os.Stat(chartPath)
	if errors.Is(err, os.ErrNotExist) {
		return "."
	}
	if stat.IsDir() {
		return chartPath
	}
	return filepath.Dir(chartPath)
}

// chartURL return URL or empty string if chart is local.
// overrideURL tacke presedence over repo.
func chartURL(repo, chartPath, overrideURL string) string {
	switch {
	case overrideURL != "":
		return overrideURL
	case repo != "":
		// 5. By chart reference and repo url: helm install --repo https://example.com/charts/ mynginx nginx
		// Note: file command s do Clean which removes // after scheme
		repo := strings.TrimSuffix(repo, "/")
		chartPath = strings.TrimPrefix(chartPath, "/")  // unlikely
		chartPath = strings.TrimPrefix(chartPath, "./") // likely
		return repo + "/" + chartPath
	// 4. By absolute URL: helm install mynginx https://example.com/charts/nginx-1.2.3.tgz
	case strings.HasPrefix(chartPath, "http"):
		return chartPath
	// 6. By OCI registries: helm install mynginx --version 1.2.3 oci://example.com/charts/nginx
	case strings.HasPrefix(chartPath, "oci"):
		return chartPath
	}
	return ""
}

// localChartPath retuns local chart path, or "" if char is URL
func localChartPath(repo, chartPath string) string {
	switch {
	case strings.HasPrefix(chartPath, "http"):
		return ""
	case strings.HasPrefix(chartPath, "oci"):
		return ""
	case repo != "":
		return ""
	}
	return chartPath
}

// shift removes top element from an arg pack
func shift(args []string) []string {
	if len(args) == 1 {
		return []string{}
	}
	return args[1:]
}
