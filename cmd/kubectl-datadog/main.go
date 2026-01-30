// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package main

import (
	"fmt"
	"log/slog"
	"os"

	"datadog.com/helm-annotate/pkg/kubectldriver"
	"datadog.com/helm-annotate/pkg/log"
	"github.com/spf13/cobra"
)

// todo: set at build time
var Version = "dev"

var (
	repoURL        string
	targetRevision string
	path           string
	verbose        bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "kubectl-datadog",
		Short:        "Add repository origin annotations to Kubernetes manifests",
		Long:         "kubectl-datadog reads Kubernetes manifests from stdin, adds repository origin annotations, and outputs to stdout.",
		Example:      "cat manifest.yaml | kubectl datadog | kubectl apply -f -",
		RunE:         runAnnotate,
		SilenceUsage: true,
		Version:      Version,
	}

	rootCmd.Flags().StringVar(&repoURL, "repo-url", "", "Repository URL (auto-detected from git if not provided)")
	rootCmd.Flags().StringVar(&targetRevision, "target-revision", "", "Target revision/commit SHA (auto-detected from git if not provided)")
	rootCmd.Flags().StringVar(&path, "path", "", "Path to folder with k8s YAML files relative to repo root")
	rootCmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose output")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runAnnotate(cmd *cobra.Command, args []string) error {
	closer := log.Configure("kubectl-datadog.log")
	defer closer.Close()

	opts := kubectldriver.Options{
		RepoURL:        repoURL,
		TargetRevision: targetRevision,
		Path:           path,
		Verbose:        verbose,
	}

	driver := kubectldriver.New(os.Stdin, os.Stdout, opts)
	if err := driver.Execute(); err != nil {
		slog.Error("Execute returned", "error", err)
		return fmt.Errorf("execute: %w", err)
	}

	return nil
}
