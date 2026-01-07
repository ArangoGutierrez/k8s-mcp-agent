// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

// Package info provides build-time version information for the agent.
package info

var (
	// gitCommit is set at build time via ldflags
	gitCommit = "unknown"
	// version is set at build time via ldflags
	version = "dev"
)

// GitCommit returns the git commit hash at build time.
func GitCommit() string {
	return gitCommit
}

// Version returns the version string at build time.
func Version() string {
	return version
}

// GetInfo returns a struct with all build information.
func GetInfo() Info {
	return Info{
		Version:   version,
		GitCommit: gitCommit,
	}
}

// Info contains build-time version information.
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
}
