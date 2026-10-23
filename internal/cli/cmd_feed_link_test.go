package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// feedLinked is a feedReady env whose settings.json links to a file in a
// dotfiles folder.
func feedLinked(t *testing.T) (*testEnv, string) {
	t.Helper()
	skipWindows(t, "symlinks need privileges on windows")
	e := feedReady(t, "")
	dir := filepath.Join(t.TempDir(), "dotfiles")
	os.MkdirAll(dir, 0o755)
	target := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(target, []byte(feedUserFile), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, e.settingsPath()); err != nil {
		t.Fatal(err)
	}
	return e, target
}

func TestFeedStatusOnALinkReportsAndExitsZero(t *testing.T) {
	e, target := feedLinked(t)

	if code := e.run("feed", "status"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if out := e.out.String(); !strings.Contains(out, "settings.json is a link to") || !strings.Contains(out, "not installed") {
		t.Errorf("%s", out)
	}
	e.reset()
	os.Remove(target)
	if code := e.run("feed", "status"); code != 0 || !strings.Contains(e.out.String(), "link") {
		t.Errorf("dangling: exit %d %s", code, e.out.String())
	}
	e.reset()
	if code := e.run("feed", "status", "--json"); code != 0 || !strings.Contains(e.out.String(), `"settingsLinkTarget"`) {
		t.Errorf("json: exit %d %s", code, e.out.String())
	}
}

func TestFeedInstallThroughALinkNamesTheRealFileAndWarnsAboutGit(t *testing.T) {
	e, target := feedLinked(t)
	os.Mkdir(filepath.Join(filepath.Dir(target), ".git"), 0o755)
	e.answer("y\n")

	if code := e.run("feed", "install"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	out := e.out.String()
	if !strings.Contains(out, "settings.json") || !strings.Contains(out, "dotfiles") || !strings.Contains(out, "git") || !strings.Contains(out, "no-op") {
		t.Errorf("missing the real path or the git warning:\n%s", out)
	}
	if fi, _ := os.Lstat(e.settingsPath()); fi.Mode()&os.ModeSymlink == 0 {
		t.Error("the link was replaced")
	}
	if b, _ := os.ReadFile(target); !strings.Contains(string(b), "clalt-feed") {
		t.Error("target not edited")
	}
	e.reset()
	if code := e.run("feed", "uninstall", "--yes"); code != 0 {
		t.Fatalf("uninstall exit %d: %s", code, e.errOut.String())
	}
	if b, _ := os.ReadFile(target); string(b) != feedUserFile {
		t.Errorf("not restored:\n%s", b)
	}
}

func TestFeedInstallThroughALinkAsksAndNoChangesNothing(t *testing.T) {
	e, target := feedLinked(t)
	e.answer("n\n")
	if code := e.run("feed", "install"); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if b, _ := os.ReadFile(target); string(b) != feedUserFile {
		t.Error("a no changed the target")
	}
}
