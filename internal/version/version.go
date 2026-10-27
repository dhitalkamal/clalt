// Package version holds build-time version metadata for clalt, injected via
// -ldflags -X at release build time. When built without ldflags (e.g. a plain
// `go install`), it falls back to runtime/debug.ReadBuildInfo so the reported
// version is still meaningful instead of the literal word "dev".
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// these are set via -ldflags "-X github.com/dhitalkamal/clalt/internal/version.Version=..."
// at release build time; Version defaults to "dev" for local/go-install builds.
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// Data is the machine-readable version payload for `clalt version --json`.
type Data struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// resolved returns the effective version and commit, falling back to
// runtime/debug build info when Version is still the unset "dev" default (a
// plain `go install` build carries no -ldflags).
func resolved() (ver, commit, date string) {
	ver, commit, date = Version, Commit, Date
	if ver != "dev" {
		return
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		ver = bi.Main.Version
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "" {
				commit = s.Value
			}
		case "vcs.time":
			if date == "" {
				date = s.Value
			}
		}
	}
	return
}

// shortCommit truncates a commit hash to 7 chars, matching git's default
// abbreviation; shorter hashes (or empty) pass through unchanged.
func shortCommit(c string) string {
	if len(c) > 7 {
		return c[:7]
	}
	return c
}

// String renders the human-readable line printed by `clalt version` and
// `clalt --version`, e.g.:
//
//	clalt v0.1.0 (commit 8ee2a33, built 2026-09-26, go1.26.6 darwin/arm64)
//
// Missing pieces (no commit, no date) are omitted gracefully.
func String() string {
	ver, commit, date := resolved()
	commit = shortCommit(commit)

	var parts []string
	if commit != "" {
		parts = append(parts, "commit "+commit)
	}
	if date != "" {
		parts = append(parts, "built "+date)
	}
	parts = append(parts, fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH))

	detail := parts[0]
	for _, p := range parts[1:] {
		detail += ", " + p
	}
	return fmt.Sprintf("clalt %s (%s)", ver, detail)
}

// Info returns the machine-readable version info for `clalt version --json`.
func Info() Data {
	ver, commit, date := resolved()
	return Data{
		Version: ver,
		Commit:  shortCommit(commit),
		Date:    date,
		Go:      runtime.Version(),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
}
