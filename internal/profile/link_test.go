package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShareConfig(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "main")
	profileDir := filepath.Join(root, "profile")

	// a fake main config: a skills dir, settings files, CLAUDE.md, plus a
	// credential and a projects dir that must be excluded.
	mustMkdir(t, filepath.Join(source, "skills"))
	mustWrite(t, filepath.Join(source, "skills", "s.md"), "x")
	mustWrite(t, filepath.Join(source, "settings.json"), `{"a":1}`)
	mustWrite(t, filepath.Join(source, "settings.local.json"), `{"b":2}`)
	mustWrite(t, filepath.Join(source, "CLAUDE.md"), "hello")
	mustWrite(t, filepath.Join(source, ".credentials.json"), `{"secret":true}`)
	mustMkdir(t, filepath.Join(source, "projects"))
	mustWrite(t, filepath.Join(source, "projects", "p.jsonl"), "log")

	done, err := ShareConfig(profileDir, source, false)
	if err != nil {
		t.Fatalf("ShareConfig: %v", err)
	}
	if len(done) == 0 {
		t.Fatal("nothing shared")
	}

	// skills is a symlink to the source
	target, err := os.Readlink(filepath.Join(profileDir, "skills"))
	if err != nil || target != filepath.Join(source, "skills") {
		t.Errorf("skills not symlinked to source: %q, %v", target, err)
	}
	// settings files are real copies, not symlinks
	for _, f := range []string{"settings.json", "settings.local.json", "CLAUDE.md"} {
		fi, err := os.Lstat(filepath.Join(profileDir, f))
		if err != nil {
			t.Fatalf("%s missing: %v", f, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%s should be a copy, not a symlink", f)
		}
	}
	// credential and projects must NOT be shared
	if exists(filepath.Join(profileDir, ".credentials.json")) {
		t.Error("credential leaked into profile")
	}
	if exists(filepath.Join(profileDir, "projects")) {
		t.Error("projects/transcripts leaked into profile")
	}
}

func TestShareConfigNoClobberWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "main")
	profileDir := filepath.Join(root, "profile")
	mustWrite(t, filepath.Join(source, "settings.json"), `{"main":true}`)
	// profile already has its own settings.json
	mustWrite(t, filepath.Join(profileDir, "settings.json"), `{"profile":true}`)

	if _, err := ShareConfig(profileDir, source, false); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, filepath.Join(profileDir, "settings.json"))
	if got != `{"profile":true}` {
		t.Errorf("profile settings.json clobbered without overwrite: %s", got)
	}

	// overwrite=true pushes the main copy over the profile's
	if _, err := ShareConfig(profileDir, source, true); err != nil {
		t.Fatal(err)
	}
	got = readFile(t, filepath.Join(profileDir, "settings.json"))
	if got != `{"main":true}` {
		t.Errorf("overwrite did not update settings.json: %s", got)
	}
}

func TestCopyDir(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	mustWrite(t, filepath.Join(src, "a.txt"), "aaa")
	mustWrite(t, filepath.Join(src, "sub", "b.txt"), "bbb")

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}
	if readFile(t, filepath.Join(dst, "a.txt")) != "aaa" {
		t.Error("top-level file not copied")
	}
	if readFile(t, filepath.Join(dst, "sub", "b.txt")) != "bbb" {
		t.Error("nested file not copied")
	}
}

func TestShareConfigMissingSourceIsNoOp(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, "profile")
	done, err := ShareConfig(profileDir, filepath.Join(root, "nope"), false)
	if err != nil {
		t.Fatalf("missing source should be a no-op: %v", err)
	}
	if len(done) != 0 {
		t.Errorf("shared %v from a missing source", done)
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
