// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Cmd is the git command executor, made public for testing
var Cmd func(args ...string) (string, error) = Exec

// Exec executes a git command with the given arguments
func Exec(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

// IsInsideRepo checks if the given directory is inside a git repository
func IsInsideRepo(dirpath string) (bool, error) {
	out, err := Cmd("-C", dirpath, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		if strings.Contains(out, "not a git repository") {
			return false, nil
		}
		return false, fmt.Errorf("git rev-parse: %w: %s", err, out)
	}
	return true, nil
}

// GetRepoURLAndSHA returns the remote origin URL and HEAD SHA for the given path
func GetRepoURLAndSHA(path string) (string, string, error) {
	url, err := Cmd("-C", path, "remote", "get-url", "origin")
	if err != nil {
		return "", "", fmt.Errorf("git remote: %w", err)
	}

	// Normalize the URL to HTTPS format
	url = normalizeGitURL(url)

	sha, err := Cmd("-C", path, "rev-parse", "HEAD")
	if err != nil {
		return "", "", fmt.Errorf("git sha: %w", err)
	}
	return url, sha, nil
}

// normalizeGitURL converts various git URL formats to HTTPS format
func normalizeGitURL(url string) string {
	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")

	// Handle different URL formats:
	// 1. ssh://git@github.com/org/repo -> https://github.com/org/repo
	// 2. git@github.com:org/repo -> https://github.com/org/repo
	// 3. https://github.com/org/repo -> https://github.com/org/repo
	// 4. http://github.com/org/repo -> https://github.com/org/repo

	// If URL already starts with http:// or https://, just normalize the protocol
	if strings.HasPrefix(url, "https://") {
		return url
	}
	if strings.HasPrefix(url, "http://") {
		return strings.Replace(url, "http://", "https://", 1)
	}

	// Handle ssh:// protocol
	if url, ok := strings.CutPrefix(url, "ssh://"); ok {
		url, _ = strings.CutPrefix(url, "git@")
		return "https://" + url
	}

	// Handle git@ SSH format (git@github.com:org/repo)
	if url, ok := strings.CutPrefix(url, "git@"); ok {
		// Replace first colon with slash (e.g., github.com:org/repo -> github.com/org/repo)
		url = strings.Replace(url, ":", "/", 1)
		return "https://" + url
	}

	// Default: assume it's already in a format we can use, just add https://
	return "https://" + url
}
