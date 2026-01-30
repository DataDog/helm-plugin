// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package helmdriver

import (
	"bytes"
	"embed"
	"errors"
	"slices"
	"strings"
	"testing"

	"datadog.com/helm-plugin/pkg/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const gitURL = "git@gitlab.com:OrgID/helm-repo.git"

//go:embed testdata/render-good.yaml
var inputChart string

//go:embed testdata/render-bad.yaml
var badInputChart string

//go:embed testdata/expected/*.yaml
var expectedFs embed.FS

func TestDriverSuccess(t *testing.T) {
	openExpected := func(n string) string {
		b, err := expectedFs.ReadFile("testdata/expected/" + n)
		require.NoError(t, err, "Unable to load embed expdecte file "+n)
		return string(b)
	}

	gitFunc1 := func(args ...string) (string, error) {
		switch {
		case slices.Contains(args, "--is-inside-work-tree"):
			return "true", nil
		case slices.Contains(args, "get-url"):
			return gitURL, nil
		case slices.Contains(args, "HEAD"):
			return "cmtsha", nil
		default:
			return "unexpected error", errors.New("unexpected error")
		}
	}

	// All test charts are given under testdata for the os. commands to work
	tests := []struct {
		name            string
		args            []string
		git             func(args ...string) (string, error)
		charURLOverride string
		pluginVersion   string
		chartBytes      string
		expected        string
	}{
		{
			name: "chart reference",
			// 1. By chart reference: helm install mynginx testdata/example/mynginx
			args:       []string{"install;mynginx;testdata/example/mynginx;-f;values.yaml"},
			git:        gitFunc1,
			chartBytes: inputChart,
			expected:   "1.yaml",
		},
		{
			name: "packaged chart",
			// 2. By path to a packaged chart: helm install mynginx ./testdata/nginx-1.2.3.tgz
			args:       []string{"install;-f;values.yaml;mynginx;./testdata/nginx-1.2.3.tgz"},
			git:        gitFunc1,
			chartBytes: inputChart,
			expected:   "2.yaml",
		},
		{
			name: "unpacked chart directory",
			// 3. By path to an unpacked chart directory: helm install mynginx ./nginx
			args:       []string{"install;mynginx;./testdata/nginx;-f;values.yaml"},
			git:        gitFunc1,
			chartBytes: inputChart,
			expected:   "3.yaml",
		},
		{
			name: "absolute URL",
			// 4. By absolute URL: helm install mynginx https://example.com/charts/nginx-1.2.3.tgz
			args:       []string{"install;mynginx;https://example.com/charts/nginx-1.2.3.tgz;--values;values-prod.yaml"},
			git:        gitFunc1,
			chartBytes: inputChart,
			expected:   "4.yaml",
		},
		{
			name: "chart reference and repo url",
			// 5. By chart reference and repo url: helm install --repo https://example.com/charts/ mynginx nginx
			args:       []string{"install;--repo;https://example.com/charts/;mynginx;nginx;--values;values-prod.yaml"},
			git:        gitFunc1,
			chartBytes: inputChart,
			expected:   "5.yaml",
		},
		{
			name: "OCI registries",
			// 6. By OCI registries: helm install mynginx --version 1.2.3 oci://example.com/charts/nginx
			args:       []string{"install;mynginx;--version;1.2.3;oci://example.com/charts/nginx;--values;values-prod.yaml"},
			git:        gitFunc1,
			chartBytes: inputChart,
			expected:   "6.yaml",
		},
		{
			name: "unpacked chart directory with override",
			// 3. By path to an unpacked chart directory: helm install mynginx ./nginx
			// wiht override
			args:            []string{"install;mynginx;./testdata/nginx;-f;values.yaml"},
			git:             gitFunc1,
			chartBytes:      inputChart,
			charURLOverride: "http://override.com/charts/nginx",
			pluginVersion:   "",
			expected:        "7.yaml",
		},
		{
			name: "chart reference with plugin version",
			// Test with plugin version set
			args:          []string{"install;mynginx;testdata/example/mynginx;-f;values.yaml"},
			git:           gitFunc1,
			chartBytes:    inputChart,
			pluginVersion: "v4/0.1.1",
			expected:      "8.yaml",
		},
		{
			name: "OCI registries with plugin version",
			// Test with plugin version set
			args:          []string{"install;mynginx;--version;1.2.3;oci://example.com/charts/nginx;--values;values-prod.yaml"},
			git:           gitFunc1,
			chartBytes:    inputChart,
			pluginVersion: "v3/1.2.3", // format is not enforced by the driver
			expected:      "9.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			git.Cmd = tt.git // override git.Cmd handler
			d := New(tt.args, strings.NewReader(tt.chartBytes), &out, tt.charURLOverride, tt.pluginVersion)
			err := d.Execute()
			require.NoError(t, err)

			output := out.String()

			// Verify plugin version annotation presence/absence
			if tt.pluginVersion != "" {
				assert.Contains(t, output, "origin.datadoghq.com/plugin-version",
					"expected plugin version annotation to be present")
			} else {
				assert.NotContains(t, output, "origin.datadoghq.com/plugin-version",
					"expected plugin version annotation to be absent")
			}

			// Verify exact output match with expected file
			assert.Equal(t, openExpected(tt.expected), output, "On "+tt.expected)
		})
	}
}

func TestChartURL(t *testing.T) {
	// func chartURL(repo, chartPath string) string {
	tests := []struct {
		name      string
		repo      string
		chartPath string
		override  string
		expected  string
	}{
		{
			"repo is given",
			"oci://example.com/charts/nginx",
			"some/chart/path",
			"",
			"oci://example.com/charts/nginx/some/chart/path",
		},
		{
			"repo is given",
			"oci://example.com/charts/nginx/",
			"some/chart/path",
			"",
			"oci://example.com/charts/nginx/some/chart/path",
		},
		{
			"chartPath is local",
			"", // repo empty
			"some/chart/path",
			"",
			"", //
		},
		{
			"chartPath is http",
			"", // repo empty
			"http://example.com/charts/nginx/some/chart/path.tgz",
			"",
			"http://example.com/charts/nginx/some/chart/path.tgz",
		},
		{
			"chartPath is oci",
			"", // repo empty
			"oci://example.com/charts/nginx/some/chart/path.tgz",
			"",
			"oci://example.com/charts/nginx/some/chart/path.tgz",
		},
		{
			"repo is given with override",
			"oci://example.com/charts/nginx/",
			"some/chart/path",
			"http://chart.com/override",
			"http://chart.com/override",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chartURL(tt.repo, tt.chartPath, tt.override)
			assert.Equal(t, tt.expected, got)
		})
	}
}
