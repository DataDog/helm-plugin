// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"datadog.com/helm-plugin/pkg/helmdriver"
	"datadog.com/helm-plugin/pkg/log"
)

func main() {
	// Define pluginver flag
	pluginVer := flag.String("pluginver", "", "Plugin version from plugin.yaml (default: no version is attached)")
	flag.Parse()

	// we expect at least 2 args
	if len(flag.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: plugin [<flags>] <helm args separated by ;>")
		flag.Usage()
		os.Exit(1)
	}
	closer := log.Configure("ddhelm.log")
	defer closer.Close()

	chartURL, _ := os.LookupEnv("DD_HELM_CHART_URL")

	// Plugin gets rendered chart over stdin and has to output to stdout
	// stdout should be valid YAML document
	driver := helmdriver.New(flag.Args(), os.Stdin, os.Stdout, chartURL, *pluginVer)
	if err := driver.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "execute: %s\n", err)
		slog.Error("Execute returned", "error", err)
		os.Exit(1)
	}
}
