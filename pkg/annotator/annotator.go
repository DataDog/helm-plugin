// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package annotator

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"datadog.com/helm-annotate/pkg/log"
	"go.yaml.in/yaml/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// DefaultAnnotationKey is the default annotation key for repository origin
	DefaultAnnotationKey = "origin.datadoghq.com/location"
	// PluginVersionAnnotationKey is the annotation key for plugin version
	PluginVersionAnnotationKey = "origin.datadoghq.com/plugin-version"
)

// Config holds the configuration for the annotator
type Config struct {
	// Input is the source of YAML manifests
	Input io.Reader
	// Output is where annotated manifests are written
	Output io.Writer

	// Verbose enables verbose logging
	Verbose bool
	// YAMLIndent controls the YAML output indentation (0 uses library default)
	YAMLIndent int
	// PluginVersion is the version of the plugin from plugin.yaml
	PluginVersion string
}

// RepoLocation is the location object for plain K8s manifests (non-Helm).
type RepoLocation struct {
	// URL is the repository URL
	URL string `json:"url,omitempty"`
	// TargetRevision is the git branch/tag/commit
	TargetRevision string `json:"targetRevision,omitempty"`
	// Path is the path to folder with k8s YAML files
	Path string `json:"path,omitempty"`
}

// HelmLocation is the location object for a Helm chart.
// It depends on install invocation:
// 1. By chart reference: helm install mymaria example/mariadb
// 2. By path to a packaged chart: helm install mynginx ./nginx-1.2.3.tgz
// 3. By path to an unpacked chart directory: helm install mynginx ./nginx
// 4. By absolute URL: helm install mynginx https://example.com/charts/nginx-1.2.3.tgz
// 5. By chart reference and repo url: helm install --repo https://example.com/charts/ mynginx nginx
// 6. By OCI registries: helm install mynginx --version 1.2.3 oci://example.com/charts/nginx
type HelmLocation struct {
	// ChartURL contains a URL chart is downloaded from. See install 4, 5, 6 cases.
	// ChartURL is empty if chart is stored in RepoURL.
	ChartURL string `json:"charURL,omitempty"`
	// RepoURL contains URL for chart and values files, if differs from ChartURL.
	// See install 1, 2, 3 cases.
	RepoURL string `json:"repoURL,omitempty"`
	// TargetRevision contains sha of the checkout branch
	TargetRevision string `json:"targetRevision,omitempty"`
	// ValuesPath contains value files used to render the chart relative to RepoURL
	ValuesPath []string `json:"valuesPath,omitempty"`
	// ChartPath is the location of the chart relative to RepoURL
	ChartPath string `json:"chartPath,omitempty"`
}

// LocationObject is the location object for a resource origin.
// It can contain either Repo (for plain K8s manifests) or Helm (for Helm charts),
// or both if applicable.
type LocationObject struct {
	// Repo is used when k8s manifest is not Helm generated, but rather plain YAML
	Repo *RepoLocation `json:"repo,omitempty"`
	// Helm is used when k8s manifest is generated from a Helm chart
	Helm *HelmLocation `json:"helm,omitempty"`
}

// IsValid checks if the location object is valid.
func (lo *LocationObject) IsValid() bool {
	return (lo.Repo != nil && lo.Helm == nil) || (lo.Helm != nil && lo.Repo == nil)
}

// Annotator handles adding repository origin annotations to Kubernetes manifests
type Annotator struct {
	config  *Config
	loc     *LocationObject
	locJSON []byte
}

// NewWithLocationObject creates a new Annotator with a location object value.
// It marshals only the relevant location type based on the Type discriminator.
func NewWithLocationObject(cfg *Config, loc LocationObject) (*Annotator, error) {
	if !loc.IsValid() {
		return nil, fmt.Errorf("location object is not valid")
	}

	locJSON, err := json.Marshal(loc)
	if err != nil {
		return nil, fmt.Errorf("marshal location object: %w", err)
	}

	log.LogVerbose(cfg.Verbose, "annotation key: %s", DefaultAnnotationKey)
	log.LogVerbose(cfg.Verbose, "annotation value: %s", string(locJSON))

	return &Annotator{
		config:  cfg,
		loc:     &loc,
		locJSON: locJSON,
	}, nil
}

// Run processes YAML documents from input, adds annotations, and writes to output
func (a *Annotator) Run() error {
	// Create YAML decoder/encoder
	decoder := yaml.NewDecoder(a.config.Input)
	encoder := yaml.NewEncoder(a.config.Output)
	defer encoder.Close()

	if a.config.YAMLIndent > 0 {
		encoder.SetIndent(a.config.YAMLIndent)
	}

	documentCount := 0

	// Process each YAML document in the stream
	for {
		var doc map[string]any
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}

			// If we reach here, we have an invalid YAML document
			return fmt.Errorf("decode YAML document %d: %w", documentCount+1, err)
		}

		// Skip empty documents
		if doc == nil {
			continue
		}

		documentCount++
		log.LogVerbose(a.config.Verbose, "processing document %d", documentCount)

		u := &unstructured.Unstructured{Object: doc}

		// Get or create annotations map and inject location annotation
		annotations := u.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}

		annotations[DefaultAnnotationKey] = string(a.locJSON)

		// Add plugin version annotation if provided
		if a.config.PluginVersion != "" {
			annotations[PluginVersionAnnotationKey] = a.config.PluginVersion
		}

		u.SetAnnotations(annotations)

		// Encode back to YAML
		if err := encoder.Encode(u.Object); err != nil {
			return fmt.Errorf("encode YAML document %d: %w", documentCount, err)
		}
	}

	log.LogVerbose(a.config.Verbose, "processed %d documents", documentCount)

	if documentCount == 0 {
		fmt.Fprintf(os.Stderr, "Warning: no valid YAML documents found in input\n")
	}

	return nil
}
