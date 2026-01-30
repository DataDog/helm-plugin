// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package git

import (
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsInsideRepo(t *testing.T) {
	tests := []struct {
		name     string
		dirpath  string
		gitOut   string
		gitErr   error
		expected bool
		wantErr  bool
	}{
		{
			name:     "inside git repo",
			dirpath:  "/some/repo/path",
			gitOut:   "true",
			gitErr:   nil,
			expected: true,
			wantErr:  false,
		},
		{
			name:     "not a git repo",
			dirpath:  "/some/path",
			gitOut:   "fatal: not a git repository (or any of the parent directories): .git",
			gitErr:   errors.New("exit status 128"),
			expected: false,
			wantErr:  false,
		},
		{
			name:     "git command fails with other error",
			dirpath:  "/some/path",
			gitOut:   "permission denied",
			gitErr:   errors.New("exit status 1"),
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock git.Cmd
			Cmd = func(args ...string) (string, error) {
				assert.Equal(t, []string{"-C", tt.dirpath, "rev-parse", "--is-inside-work-tree"}, args)
				return tt.gitOut, tt.gitErr
			}

			result, err := IsInsideRepo(tt.dirpath)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetRepoURLAndSHA(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		gitURLOut   string // output from "git remote get-url origin"
		gitSHAOut   string // output from "git rev-parse HEAD"
		gitURLErr   error
		gitSHAErr   error
		expectedURL string
		expectedSHA string
		wantErr     bool
	}{
		{
			name:        "SSH format with .git suffix",
			path:        "/repo",
			gitURLOut:   "git@github.com:DataDog/helm-plugin.git",
			gitSHAOut:   "abc123def456",
			expectedURL: "https://github.com/DataDog/helm-plugin",
			expectedSHA: "abc123def456",
			wantErr:     false,
		},
		{
			name:        "SSH format without .git suffix",
			path:        "/repo",
			gitURLOut:   "git@github.com:DataDog/helm-plugin",
			gitSHAOut:   "abc123def456",
			expectedURL: "https://github.com/DataDog/helm-plugin",
			expectedSHA: "abc123def456",
			wantErr:     false,
		},
		{
			name:        "SSH format with gitlab",
			path:        "/repo",
			gitURLOut:   "git@gitlab.com:myorg/myrepo.git",
			gitSHAOut:   "xyz789",
			expectedURL: "https://gitlab.com/myorg/myrepo",
			expectedSHA: "xyz789",
			wantErr:     false,
		},
		{
			name:        "HTTPS format with .git suffix",
			path:        "/repo",
			gitURLOut:   "https://github.com/DataDog/helm-plugin.git",
			gitSHAOut:   "abc123def456",
			expectedURL: "https://github.com/DataDog/helm-plugin",
			expectedSHA: "abc123def456",
			wantErr:     false,
		},
		{
			name:        "HTTPS format without .git suffix",
			path:        "/repo",
			gitURLOut:   "https://github.com/DataDog/helm-plugin",
			gitSHAOut:   "abc123def456",
			expectedURL: "https://github.com/DataDog/helm-plugin",
			expectedSHA: "abc123def456",
			wantErr:     false,
		},
		{
			name:        "HTTP format with .git suffix",
			path:        "/repo",
			gitURLOut:   "http://github.com/DataDog/helm-plugin.git",
			gitSHAOut:   "abc123def456",
			expectedURL: "https://github.com/DataDog/helm-plugin",
			expectedSHA: "abc123def456",
			wantErr:     false,
		},
		{
			name:        "HTTP format without .git suffix",
			path:        "/repo",
			gitURLOut:   "http://github.com/DataDog/helm-plugin",
			gitSHAOut:   "abc123def456",
			expectedURL: "https://github.com/DataDog/helm-plugin",
			expectedSHA: "abc123def456",
			wantErr:     false,
		},
		{
			name:        "ssh:// protocol format",
			path:        "/repo",
			gitURLOut:   "ssh://git@github.com/DataDog/helm-plugin.git",
			gitSHAOut:   "abc123def456",
			expectedURL: "https://github.com/DataDog/helm-plugin",
			expectedSHA: "abc123def456",
			wantErr:     false,
		},
		{
			name:        "nested path with SSH format",
			path:        "/repo",
			gitURLOut:   "git@github.com:DataDog/helm-charts/datadog.git",
			gitSHAOut:   "def456",
			expectedURL: "https://github.com/DataDog/helm-charts/datadog",
			expectedSHA: "def456",
			wantErr:     false,
		},
		{
			name:        "git remote command fails",
			path:        "/repo",
			gitURLOut:   "fatal: not a git repository",
			gitURLErr:   errors.New("exit status 128"),
			expectedURL: "",
			expectedSHA: "",
			wantErr:     true,
		},
		{
			name:        "git rev-parse command fails",
			path:        "/repo",
			gitURLOut:   "git@github.com:DataDog/helm-plugin.git",
			gitSHAOut:   "fatal: ambiguous argument 'HEAD'",
			gitSHAErr:   errors.New("exit status 128"),
			expectedURL: "",
			expectedSHA: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock git.Cmd
			Cmd = func(args ...string) (string, error) {
				switch {
				case slices.Contains(args, "get-url"):
					assert.Equal(t, []string{"-C", tt.path, "remote", "get-url", "origin"}, args)
					return tt.gitURLOut, tt.gitURLErr
				case slices.Contains(args, "HEAD"):
					assert.Equal(t, []string{"-C", tt.path, "rev-parse", "HEAD"}, args)
					return tt.gitSHAOut, tt.gitSHAErr
				default:
					t.Fatalf("unexpected git command: %v", args)
					return "", errors.New("unexpected command")
				}
			}

			url, sha, err := GetRepoURLAndSHA(tt.path)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedURL, url, "URL mismatch")
				assert.Equal(t, tt.expectedSHA, sha, "SHA mismatch")
			}
		})
	}
}
