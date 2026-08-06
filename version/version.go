// Package version carries the build-time semver of the matrix-log-chat
// binary. The value is injected via -ldflags "-X paepcke.de/matrix-log-chat/version.Version=..."
// by the Makefile (see the VERSION / LDFLAGS variables). When the binary is
// built outside the Makefile the string defaults to "v0.0.0-dev" so the
// binary always reports something meaningful.
package version

// Version is the semantic version of the running binary. It is overwritten
// at link time by the Makefile. The format follows semver: vMAJOR.MINOR.PATCH
// (optionally with a -prerelease suffix). Patch is the only segment bumped by
// the project's release flow (see AGENTS.md, "Tag" step).
var Version = "v0.0.0-dev"

// Build is an optional build metadata string (commit hash, dirty tree, etc.)
// that the Makefile may inject alongside Version via -X. Empty when unset.
var Build = ""

// String returns "Version (Build)" when Build is set, otherwise just Version.
func String() string {
	if Build != "" {
		return Version + " (" + Build + ")"
	}
	return Version
}
