package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realDir makes a directory and returns its symlink-free path, since temp dirs
// on macOS sit under /var -> /private/var.
func realDir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestResolvePathFollowsSymlinks(t *testing.T) {
	root := t.TempDir()
	real := realDir(t, filepath.Join(root, "real"))
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if got := ResolvePath(link); got != real {
		t.Errorf("ResolvePath(link) = %q, want %q", got, real)
	}
}

func TestResolvePathKeepsMissingTail(t *testing.T) {
	root := t.TempDir()
	real := realDir(t, filepath.Join(root, "real"))
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got := ResolvePath(filepath.Join(link, "not", "yet"))
	want := filepath.Join(real, "not", "yet")
	if got != want {
		t.Errorf("ResolvePath = %q, want %q", got, want)
	}
}

func TestResolvePathCleansAndMakesAbsolute(t *testing.T) {
	root := realDir(t, t.TempDir())
	t.Chdir(root)
	if got := ResolvePath("sub/../x/"); got != filepath.Join(root, "x") {
		t.Errorf("relative: got %q, want %q", got, filepath.Join(root, "x"))
	}
	if got := ResolvePath(root + string(filepath.Separator)); got != root {
		t.Errorf("trailing separator: got %q, want %q", got, root)
	}
}

func TestResolvePathLeavesDanglingLinkAlone(t *testing.T) {
	root := t.TempDir()
	dir := realDir(t, filepath.Join(root, "dir"))
	if err := os.Symlink(filepath.Join(root, "gone"), filepath.Join(dir, "dangling")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got := ResolvePath(filepath.Join(dir, "dangling"))
	if want := filepath.Join(dir, "dangling"); got != want {
		t.Errorf("ResolvePath = %q, want %q", got, want)
	}
}

func TestSamePath(t *testing.T) {
	root := t.TempDir()
	real := realDir(t, filepath.Join(root, "real"))
	other := realDir(t, filepath.Join(root, "other"))
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if !SamePath(real, link) {
		t.Error("a symlink to a folder should be the same path as the folder")
	}
	if !SamePath(filepath.Join(link, "new"), filepath.Join(real, "new", "..", "new")) {
		t.Error("paths that do not exist yet should compare by their resolved prefix")
	}
	if SamePath(real, other) {
		t.Error("different folders compared equal")
	}
}

func TestEmptyPathIsNeverSame(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if got := ResolvePath(""); got != "" {
		t.Errorf("ResolvePath(\"\") = %q, want empty", got)
	}
	// an empty path must not quietly turn into the working directory
	if SamePath("", root) || SamePath("", "") {
		t.Error("an empty path should never match anything")
	}
}

// caseInsensitive reports whether dir sits on a file system that treats names
// differing only in case as one entry (the default on macOS and Windows).
func caseInsensitive(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "CaseProbe")
	if err := os.Mkdir(probe, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := os.Stat(filepath.Join(dir, "caseprobe"))
	return err == nil
}

func TestSamePathComparesFileIdentity(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	if err := os.WriteFile(a, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(a, b); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	other := filepath.Join(root, "other")
	if err := os.WriteFile(other, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// two names for one file resolve to different strings
	if !SamePath(a, b) {
		t.Error("two hard links to one file should be the same path")
	}
	if SamePath(a, other) {
		t.Error("two files with equal content are not the same path")
	}
}

func TestSamePathIgnoresCaseWhereTheFileSystemDoes(t *testing.T) {
	root := realDir(t, t.TempDir())
	if !caseInsensitive(t, root) {
		t.Skip("case-sensitive file system")
	}
	if !SamePath(filepath.Join(root, "CaseProbe"), filepath.Join(root, "caseprobe")) {
		t.Error("names that differ only in case are one folder here")
	}
}

func TestWithin(t *testing.T) {
	root := t.TempDir()
	dir := realDir(t, filepath.Join(root, "clalt"))
	if err := os.MkdirAll(filepath.Join(dir, "accounts", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	// filepath.Join cleans its result, so a ".." has to be spelled out by hand to
	// reach Within as written
	raw := func(parts ...string) string { return strings.Join(parts, string(filepath.Separator)) }
	cases := []struct {
		name   string
		p, dir string
		want   bool
	}{
		{"below", filepath.Join(dir, "accounts", "a"), dir, true},
		{"the folder itself", dir, dir, true},
		{"not created yet", filepath.Join(dir, "accounts", "new", "deeper"), dir, true},
		{"through a symlink", filepath.Join(link, "accounts", "a"), dir, true},
		{"dir through a symlink", filepath.Join(dir, "accounts"), link, true},
		{"dotdot that stays inside", raw(dir, "accounts", "x", "..", "a"), dir, true},
		{"dotdot that leaves", raw(dir, "accounts", "..", "..", "elsewhere"), dir, false},
		{"shared prefix only", dir + "-backup", dir, false},
		{"parent of the folder", root, dir, false},
		{"empty p", "", dir, false},
		{"empty dir", dir, "", false},
	}
	for _, c := range cases {
		if got := Within(c.p, c.dir); got != c.want {
			t.Errorf("%s: Within(%q, %q) = %v, want %v", c.name, c.p, c.dir, got, c.want)
		}
	}
}

func TestWithinWhenTheFolderDoesNotExist(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "never-made")
	if !Within(filepath.Join(dir, "accounts", "a"), dir) {
		t.Error("a path under a missing folder should still be inside it")
	}
	if Within(filepath.Join(root, "other"), dir) {
		t.Error("a path outside a missing folder is not inside it")
	}
}

func TestWithinIgnoresCaseWhereTheFileSystemDoes(t *testing.T) {
	root := realDir(t, t.TempDir())
	if !caseInsensitive(t, root) {
		t.Skip("case-sensitive file system")
	}
	inner := filepath.Join(root, "CaseProbe", "accounts", "a")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if !Within(filepath.Join(root, "caseprobe", "accounts", "a"), filepath.Join(root, "CaseProbe")) {
		t.Error("a differently cased spelling of a folder below it is still inside it")
	}
}
