package reviews

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// gitT runs a setup git command for a test and returns its trimmed stdout.
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeT(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// the real git runs the allowlisted argv against a clone that is out of date
// (the base commit is not in it yet), under a global config that would break a
// porcelain git diff. the diff must be the pull request's changes only, with
// plain a/ b/ headers.
func TestLocalDiffWithRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
	root := t.TempDir()
	hostile := filepath.Join(root, "gitconfig")
	writeT(t, hostile, "[diff]\n\tnoprefix = true\n\tmnemonicPrefix = true\n\texternal = false\n[color]\n\tui = always\n")
	t.Setenv("GIT_CONFIG_GLOBAL", hostile)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	up := filepath.Join(root, "up")
	if err := os.Mkdir(up, 0o700); err != nil {
		t.Fatal(err)
	}
	gitT(t, up, "init", "-q", "-b", "main")
	writeT(t, filepath.Join(up, "a.txt"), "a\n")
	gitT(t, up, "add", ".")
	gitT(t, up, "commit", "-qm", "base")
	gitT(t, up, "checkout", "-qb", "feat")
	writeT(t, filepath.Join(up, "a.txt"), "a\nb\n")
	gitT(t, up, "mv", "a.txt", "moved.txt")
	writeT(t, filepath.Join(up, "new.txt"), "n\n")
	gitT(t, up, "add", ".")
	gitT(t, up, "commit", "-qm", "feat")
	gitT(t, up, "checkout", "-q", "main")
	writeT(t, filepath.Join(up, "c.txt"), "c\n")
	gitT(t, up, "add", ".")
	gitT(t, up, "commit", "-qm", "main moves on")

	clone := filepath.Join(root, "clone")
	gitT(t, root, "clone", "-q", up, clone)
	// the base moves again after the clone, so the clone lacks it
	writeT(t, filepath.Join(up, "d.txt"), "d\n")
	gitT(t, up, "add", ".")
	gitT(t, up, "commit", "-qm", "main moves again")
	base := gitT(t, up, "rev-parse", "main")

	ctx := context.Background()
	g := Git{X: RealExec{}}
	wt := filepath.Join(root, "wt")
	if err := g.AddWorktree(ctx, clone, wt); err != nil {
		t.Fatal(err)
	}
	gitT(t, wt, "checkout", "-q", "--detach", "origin/feat")
	if err := g.FetchCommit(ctx, wt, base); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	diff, err := g.DiffFromBase(ctx, wt, base)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if strings.Contains(diff, "\x1b[") {
		t.Errorf("diff has color codes: %q", diff)
	}
	for _, want := range []string{"diff --git a/a.txt b/moved.txt\n", "rename from a.txt\n", "+b\n", "diff --git a/new.txt b/new.txt\n"} {
		if !strings.Contains(diff, want) {
			t.Errorf("diff lacks %q:\n%s", want, diff)
		}
	}
	for _, not := range []string{"diff --git a/c.txt", "diff --git a/d.txt"} {
		if strings.Contains(diff, not) {
			t.Errorf("diff has %s, a base change, not the pull request's:\n%s", not, diff)
		}
	}
	_, secs := cutSections(diff)
	var names []string
	for _, s := range secs {
		names = append(names, sectionName(s))
	}
	if !slices.Equal(names, []string{"moved.txt", "new.txt"}) {
		t.Errorf("chunker sees files %v", names)
	}
}
