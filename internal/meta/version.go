// Package meta exposes build-time version and user-agent information.
package meta

import "fmt"

// These are overridden at build time via -ldflags.
var (
	Version       = ""
	GitCommit     = ""
	UserAgentBase = "s1temap checker"
)

// UserAgent returns the HTTP User-Agent string for outgoing requests.
func UserAgent() string {
	return fmt.Sprintf("%s (%s)", UserAgentBase, VersionString())
}

// VersionString returns the build version and short commit, e.g. "dev" or
// "1.2.3-abc1234". It has no side effects.
func VersionString() string {
	version := Version
	if version == "" {
		version = "dev"
	}

	commit := ""
	if len(GitCommit) >= 7 {
		commit = "-" + GitCommit[:7]
	}

	return version + commit
}
