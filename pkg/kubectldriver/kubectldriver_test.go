// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package kubectldriver

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

const gitURL = "git@gitlab.com:OrgID/k8s-repo.git"

//go:embed testdata/input-good.yaml
var inputManifests string

//go:embed testdata/input-bad.yaml
var badInputManifests string

//go:embed testdata/expected/*.yaml
var expectedFs embed.FS

func TestDriverSuccess(t *testing.T) {
	openExpected := func(n string) string {
		b, err := expectedFs.ReadFile("testdata/expected/" + n)
		require.NoError(t, err, "Unable to load embed expected file "+n)
		return string(b)
	}

	gitFunc := func(args ...string) (string, error) {
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

	tests := []struct {
		name     string
		opts     Options
		git      func(args ...string) (string, error)
		input    string
		expected string
	}{
		{
			name: "explicit all options",
			opts: Options{
				RepoURL:        "https://github.com/myorg/myrepo",
				TargetRevision: "abc123",
				Path:           "k8s/overlays/prod",
			},
			git:      gitFunc,
			input:    inputManifests,
			expected: "1.yaml",
		},
		{
			name: "explicit repo and target revision, no path",
			opts: Options{
				RepoURL:        "https://github.com/myorg/myrepo",
				TargetRevision: "abc123",
			},
			git:      gitFunc,
			input:    inputManifests,
			expected: "2.yaml",
		},
		{
			name: "auto detect repo and target revision from git",
			opts: Options{
				Path: "manifests",
			},
			git:      gitFunc,
			input:    inputManifests,
			expected: "3.yaml",
		},
		{
			name: "auto detect target revision from git",
			opts: Options{
				RepoURL: "https://github.com/explicit/repo",
				Path:    "custom/path",
			},
			git:      gitFunc,
			input:    inputManifests,
			expected: "4.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			git.Cmd = tt.git // override git.Cmd handler
			d := New(strings.NewReader(tt.input), &out, tt.opts)
			err := d.Execute()
			require.NoError(t, err)
			assert.Equal(t, openExpected(tt.expected), out.String())
		})
	}
}

func TestDriverErrors(t *testing.T) {
	gitFunc := func(args ...string) (string, error) {
		if slices.Contains(args, "--is-inside-work-tree") {
			return "fatal: not a git repository", errors.New("exit status 128")
		}
		return "unexpected error", errors.New("unexpected error")
	}

	tests := []struct {
		name  string
		opts  Options
		git   func(args ...string) (string, error)
		input string
	}{
		{
			name:  "not in git repo and no repo-url",
			opts:  Options{},
			git:   gitFunc,
			input: inputManifests,
		},
		{
			name: "not in git repo and no target-revision",
			opts: Options{
				RepoURL: "https://github.com/myorg/myrepo",
			},
			git:   gitFunc,
			input: inputManifests,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			git.Cmd = tt.git
			d := New(strings.NewReader(tt.input), &out, tt.opts)
			err := d.Execute()
			// All tests should error
			require.Error(t, err, "expected error but got none")
		})
	}
}

func TestDriverWithBadInput(t *testing.T) {
	gitFunc := func(args ...string) (string, error) {
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

	tests := []struct {
		name  string
		opts  Options
		git   func(args ...string) (string, error)
		input string
	}{
		{
			name: "fail on invalid yaml",
			opts: Options{
				RepoURL:        "https://github.com/myorg/myrepo",
				TargetRevision: "abc123",
			},
			git:   gitFunc,
			input: badInputManifests,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			git.Cmd = tt.git
			d := New(strings.NewReader(tt.input), &out, tt.opts)
			err := d.Execute()
			// All tests should error
			require.Error(t, err, "expected error but got none")
		})
	}
}
