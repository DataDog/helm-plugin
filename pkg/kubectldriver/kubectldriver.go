// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package kubectldriver

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"

	"datadog.com/helm-plugin/pkg/annotator"
	"datadog.com/helm-plugin/pkg/git"
)

var _ Driver = (*driver)(nil)

// Driver interface for the kubectl datadog plugin
type Driver interface {
	Execute() error
}

// Options for configuring the driver
type Options struct {
	// RepoURL is the repository URL (optional, auto-detected if not provided)
	RepoURL string
	// TargetRevision is the git commit/branch/tag (optional, auto-detected if not provided)
	TargetRevision string
	// Path is the path to k8s YAML files relative to repo root (optional)
	Path string
	// Verbose enables verbose logging
	Verbose bool
}

type driver struct {
	in   io.Reader
	out  io.Writer
	opts Options
	loc  *annotator.LocationObject
}

// New creates a new kubectl datadog driver
func New(in io.Reader, out io.Writer, opts Options) Driver {
	return &driver{
		in:   in,
		out:  out,
		opts: opts,
		loc: &annotator.LocationObject{
			Repo: &annotator.RepoLocation{},
		},
	}
}

// Execute runs the annotation process
func (d *driver) Execute() error {
	slog.Debug("Execute", "opts", d.opts)

	// Build location info from options or auto-detect
	if err := d.buildLocation(); err != nil {
		return fmt.Errorf("build location: %w", err)
	}

	// Read all stdin into a buffer
	var input bytes.Buffer
	if _, err := io.Copy(&input, d.in); err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		return fmt.Errorf("io copy: %w", err)
	}

	// Run the annotator
	if err := d.run(&input); err != nil {
		fmt.Fprintf(os.Stderr, "annotate error: %v\n", err)
		return fmt.Errorf("run: %w", err)
	}

	return nil
}

// buildLocation builds the location object from options or auto-detects from git
func (d *driver) buildLocation() error {
	repoURL := d.opts.RepoURL
	targetRevision := d.opts.TargetRevision

	// Only auto-detect if we need to
	needAutoDetect := repoURL == "" || targetRevision == ""

	if needAutoDetect {
		isRepo, err := git.IsInsideRepo(".")
		if err != nil {
			// Not in a git repo
			if repoURL == "" {
				return fmt.Errorf("not inside a git repository and --repo-url not provided")
			}
			if targetRevision == "" {
				return fmt.Errorf("not inside a git repository and --target-revision not provided")
			}
		} else if isRepo {
			// Get git info
			url, sha, err := git.GetRepoURLAndSHA(".")
			if err != nil {
				return fmt.Errorf("git.GetRepoURLAndSHA: %w", err)
			}

			if repoURL == "" {
				repoURL = url
				slog.Debug("Auto-detected repo URL", "url", repoURL)
			}
			if targetRevision == "" {
				targetRevision = sha
				slog.Debug("Auto-detected target revision", "sha", targetRevision)
			}
		} else {
			// Not in a git repo
			if repoURL == "" {
				return fmt.Errorf("not inside a git repository and --repo-url not provided")
			}
			if targetRevision == "" {
				return fmt.Errorf("not inside a git repository and --target-revision not provided")
			}
		}
	}

	d.loc.Repo.URL = repoURL
	d.loc.Repo.TargetRevision = targetRevision
	d.loc.Repo.Path = d.opts.Path

	slog.Debug("Location built", "url", repoURL, "revision", targetRevision, "path", d.opts.Path)

	return nil
}

// run processes the manifests and adds annotations
func (d *driver) run(in *bytes.Buffer) error {
	cfg := &annotator.Config{
		Input:   in,
		Output:  d.out,
		Verbose: d.opts.Verbose,
	}

	ann, err := annotator.NewWithLocationObject(cfg, *d.loc)
	if err != nil {
		return fmt.Errorf("create annotator: %w", err)
	}

	if err := ann.Run(); err != nil {
		return fmt.Errorf("annotator run: %w", err)
	}

	return nil
}
