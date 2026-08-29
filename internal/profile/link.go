package profile

import (
	"io"
	"os"
	"path/filepath"
)

// shareExclude names entries under the user's main Claude config that must NOT
// be shared into a profile: the live credential (each profile carries its own),
// and per-machine session/telemetry state that would cross-contaminate accounts
// or leak one account's history into another.
var shareExclude = map[string]bool{
	".credentials.json": true, // the main login secret; never share it
	"projects":          true, // transcripts and session history
	"statsig":           true, // telemetry state
	"todos":             true, // per-session todo state
	"logs":              true,
	"shell-snapshots":   true,
	"history.jsonl":     true,
	"ide":               true,
	".claude.json":      true, // MCP is projected separately via LinkMCP
}

// ShareConfig makes the user's main Claude config available inside a profile so
// a `clalt run` session behaves like their normal setup: every top-level
// directory is symlinked (large and effectively read-only during a session) and
// every top-level regular file is copied (small, so a profile can diverge).
// Credentials and per-session state are excluded (see shareExclude). Existing
// entries in the profile are never clobbered unless overwrite is set, which
// re-copies files so an updated main config can be pushed with `run --refresh`.
// A missing source (a standalone setup with no main config) is a no-op. Returns
// the names shared.
func ShareConfig(profileDir, source string, overwrite bool) ([]string, error) {
	if !isDir(source) {
		return nil, nil // no main config to share
	}
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return nil, err
	}
	var done []string
	for _, e := range entries {
		name := e.Name()
		if shareExclude[name] {
			continue
		}
		src := filepath.Join(source, name)
		dst := filepath.Join(profileDir, name)

		if e.IsDir() {
			if exists(dst) {
				continue // never replace a profile's own dir or existing link
			}
			if err := os.Symlink(src, dst); err != nil {
				// symlinks need privilege on Windows; fall back to a copy so the
				// dir still carries over there.
				if cerr := copyDir(src, dst); cerr != nil {
					continue // best-effort
				}
			}
			done = append(done, name+"/")
			continue
		}
		if !e.Type().IsRegular() {
			continue // skip sockets, symlinks, and other odd entries
		}
		if exists(dst) && !overwrite {
			continue // do not clobber a profile's own file
		}
		if err := copyFile(src, dst); err != nil {
			continue
		}
		done = append(done, name)
	}
	return done, nil
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// copyDir recursively copies src into dst, used as the fallback when a symlink
// cannot be created (notably on Windows without the symlink privilege).
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !d.Type().IsRegular() {
			return nil // skip non-regular entries
		}
		return copyFile(path, target)
	})
}
