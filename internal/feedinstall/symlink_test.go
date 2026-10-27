package feedinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// linkRig is a rig whose settings.json is a symlink to a file in another
// folder, the way a dotfiles repo shares it.
func linkRig(t *testing.T, body string) (*rig, string) {
	t.Helper()
	if isWindows() {
		t.Skip("symlinks need privileges on windows")
	}
	r := newRig(t, "")
	target := filepath.Join(t.TempDir(), "dotfiles", "settings.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(body), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, r.settings); err != nil {
		t.Fatal(err)
	}
	return r, target
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assertStillLink(t *testing.T, link, target string) {
	t.Helper()
	fi, err := os.Lstat(link)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was replaced: %v", err)
	}
	if got, _ := os.Readlink(link); got != target {
		t.Errorf("link points at %s, want %s", got, target)
	}
}

func TestInstallThroughALinkEditsTheTarget(t *testing.T) {
	r, target := linkRig(t, userFile)

	p := r.install()

	assertStillLink(t, r.settings, target)
	got := readFile(t, target)
	if !strings.Contains(got, "clalt-feed") || !strings.Contains(got, `"model": "opus"`) {
		t.Errorf("target not edited:\n%s", got)
	}
	if fi, _ := os.Stat(target); fi.Mode().Perm() != 0o640 {
		t.Errorf("mode %v", fi.Mode().Perm())
	}
	if b := readFile(t, p.BackupPath); b != userFile {
		t.Errorf("backup is not the target's old bytes:\n%s", b)
	}
	if st, _ := loadState(r.opts.StatePath); st.Target == "" || !strings.HasSuffix(st.Target, "settings.json") {
		t.Errorf("state target %q", st.Target)
	}
	if p.Target().Link != true {
		t.Error("plan does not say it went through a link")
	}
}

func TestUninstallThroughALinkRestoresTheTargetExactly(t *testing.T) {
	r, target := linkRig(t, userFile)
	r.install()

	res, err := Uninstall(r.opts)

	if err != nil || !res.Restored {
		t.Fatalf("%+v %v", res, err)
	}
	assertStillLink(t, r.settings, target)
	if got := readFile(t, target); got != userFile {
		t.Errorf("target not restored:\n%s", got)
	}
}

func TestUninstallRefusesWhenTheLinkMovedSinceInstall(t *testing.T) {
	r, target := linkRig(t, userFile)
	r.install()
	other := filepath.Join(t.TempDir(), "other.json")
	os.WriteFile(other, []byte("{}"), 0o600)
	os.Remove(r.settings)
	os.Symlink(other, r.settings)
	before := readFile(t, target)

	_, err := Uninstall(r.opts)

	if err == nil || !strings.Contains(err.Error(), "points somewhere else") {
		t.Fatalf("%v", err)
	}
	if readFile(t, target) != before || readFile(t, other) != "{}" {
		t.Error("something was edited")
	}
}

func TestResolveRefusals(t *testing.T) {
	t.Run("dangling", func(t *testing.T) {
		r, target := linkRig(t, "{}")
		os.Remove(target)
		if _, err := Prepare(r.opts); err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("%v", err)
		}
	})
	t.Run("loop", func(t *testing.T) {
		r, _ := linkRig(t, "{}")
		os.Remove(r.settings)
		other := filepath.Join(filepath.Dir(r.settings), "b.json")
		os.Symlink(other, r.settings)
		os.Symlink(r.settings, other)
		if _, err := Prepare(r.opts); err == nil || !strings.Contains(err.Error(), "loop") {
			t.Errorf("%v", err)
		}
	})
	t.Run("not a regular file", func(t *testing.T) {
		r, _ := linkRig(t, "{}")
		os.Remove(r.settings)
		os.Symlink(t.TempDir(), r.settings)
		if _, err := Prepare(r.opts); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Errorf("%v", err)
		}
	})
	t.Run("inside clalt's own folder", func(t *testing.T) {
		r, target := linkRig(t, "{}")
		r.opts.Protected = []string{filepath.Dir(target)}
		if _, err := Prepare(r.opts); err == nil || !strings.Contains(err.Error(), "clalt's own") {
			t.Errorf("%v", err)
		}
	})
	t.Run("owned by someone else", func(t *testing.T) {
		r, _ := linkRig(t, "{}")
		restore := fakeOwner(false)
		defer restore()
		if _, err := Prepare(r.opts); err == nil || !strings.Contains(err.Error(), "not yours") {
			t.Errorf("%v", err)
		}
	})
}

func TestTargetInAGitTreeIsReported(t *testing.T) {
	r, target := linkRig(t, "{}")
	repo := filepath.Dir(target)
	os.Mkdir(filepath.Join(repo, ".git"), 0o755)

	tg, err := ResolveTarget(r.opts)

	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(repo)
	if tg.GitRoot != want {
		t.Errorf("git root %q, want %q", tg.GitRoot, want)
	}
}

func TestCheckOnALinkNeverFails(t *testing.T) {
	r, target := linkRig(t, userFile)
	st, err := Check(r.opts)
	if err != nil || !st.Link || st.Installed {
		t.Fatalf("%+v %v", st, err)
	}
	r.install()
	if st, err = Check(r.opts); err != nil || !st.Installed || st.Hooks == 0 {
		t.Errorf("after install: %+v %v", st, err)
	}
	os.Remove(target)
	st, err = Check(r.opts)
	if err != nil || !st.Link || st.LinkProblem == "" {
		t.Errorf("dangling: %+v %v", st, err)
	}
}
