// Package buildinfo holds version and build metadata set via -ldflags.
package buildinfo

import "fmt"

var (
	// Version is the release version (e.g. "0.1.0"), set via -ldflags.
	Version = "dev"
	// GitCommit is the short git SHA, set via -ldflags.
	GitCommit = "unknown"
	// GitDirty is "true" if the working tree had uncommitted changes at build time.
	GitDirty = "false"
	// BuildType is "release" or "debug".
	BuildType = "debug"
)

// String returns a human-readable version string.
func String() string {
	v := Version + " (" + GitCommit + ")"
	if GitDirty == "true" {
		v += "-dirty"
	}
	v += " [" + BuildType + "]"
	return v
}

// Short returns just version and commit, e.g. "v0.1.0 (a1b2c3d)".
func Short() string {
	return fmt.Sprintf("v%s (%s)", Version, GitCommit)
}
