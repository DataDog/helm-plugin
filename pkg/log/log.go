// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package log

import (
	"io"
	"log/slog"
	"os"
)

func Configure(name string) io.Closer {
	// Only enable logging is DEBUG is on
	debug := os.Getenv("HELM_DEBUG")
	if debug == "" || debug == "false" {
		return io.NopCloser(nil)
	}

	// Open (or create) the log file
	logFile, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644) // |os.O_APPEND
	if err != nil {
		panic(err)
	}

	// Create a logger using TextHandler or JSONHandler
	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo, // optional
	}))

	// Replace the default logger (optional)
	slog.SetDefault(logger)

	// Example logs
	slog.Info("Application started")

	return logFile
}

// LogVerbose logs a message if verbose mode is enabled
func LogVerbose(verbose bool, format string, args ...any) {
	if verbose {
		slog.Debug(format, args...)
	}
}
