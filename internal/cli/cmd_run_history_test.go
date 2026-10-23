package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unmountHistory turns the shared projects folder into a link to a place that is
// not there, which is what a folder on an unmounted volume looks like.
func unmountHistory(t *testing.T, e *testEnv) {
	t.Helper()
	projects := filepath.Join(e.home, ".claude", "projects")
	if err := os.RemoveAll(projects); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, filepath.Join(e.home, "volume-that-is-not-mounted", "projects"), projects)
}

// remountHistory puts a real projects folder back where the link was.
func remountHistory(t *testing.T, e *testEnv) {
	t.Helper()
	projects := filepath.Join(e.home, ".claude", "projects")
	if err := os.Remove(projects); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projects, "-p"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func wantHistoryRefusal(t *testing.T, e *testEnv, code int) {
	t.Helper()
	msg := e.errOut.String()
	for _, want := range []string{"history would not be shared", "not reachable", "unmounted", "clalt doctor"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal is missing %q:\n%s", want, msg)
		}
	}
	if code != 1 || e.claude.sessionStarted() {
		t.Errorf("exit %d, session started %v", code, e.claude.sessionStarted())
	}
}

// claude makes a real projects folder when there is none, and the account's
// history would then be its own. so an account made while the shared history was
// unmounted, which the planner leaves without a link and without a word, is
// refused at run.
func TestRunRefusesAnAccountMadeWhileTheSharedHistoryWasUnmounted(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	unmountHistory(t, e)
	e.signIn("work", "a@example.com")
	if exists(filepath.Join(e.folder("work"), "projects")) {
		t.Fatal("the planner linked an unmounted folder, which this test is not about")
	}

	e.reset()
	code := e.run("run", "work")

	wantHistoryRefusal(t, e, code)
	if exists(filepath.Join(e.folder("work"), "projects")) {
		t.Error("run left something at projects for claude to fill")
	}
	if got := e.account("work").Meta.LastUsedAt; !got.IsZero() {
		t.Errorf("lastUsedAt = %v, a refused run is not a use", got)
	}
}

func TestRunRefusesAnAccountWhoseSharedHistoryWentAway(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	unmountHistory(t, e)

	e.reset()
	code := e.run("run", "work")

	wantHistoryRefusal(t, e, code)
	if strings.Contains(e.errOut.String(), "move what is in it") {
		t.Errorf("there is nothing in the account to move:\n%s", e.errOut.String())
	}
	if fi, err := os.Lstat(filepath.Join(e.folder("work"), "projects")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the account's link was changed: %v", err)
	}
}

func TestRunWorksAgainOnceTheSharedHistoryIsBack(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	unmountHistory(t, e)
	e.signIn("work", "a@example.com")
	e.reset()
	if code := e.run("run", "work"); code != 1 {
		t.Fatalf("exit %d, want the refusal first: %s", code, e.errOut.String())
	}

	remountHistory(t, e)
	e.reset()
	code := e.run("run", "work")

	if code != 0 || !e.claude.sessionStarted() {
		t.Errorf("exit %d, started %v: %s", code, e.claude.sessionStarted(), e.errOut.String())
	}
	link, err := os.Readlink(filepath.Join(e.folder("work"), "projects"))
	if err != nil || link != filepath.Join(e.home, ".claude", "projects") {
		t.Errorf("projects links to %q (%v)", link, err)
	}
}

// whatever the planner did, the folder is looked at.
func TestHistoryOf(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, folder, shared string)
		want  historyState
	}{
		{"a link to the shared projects folder", func(t *testing.T, folder, shared string) {
			mustSymlink(t, filepath.Join(shared, "projects"), filepath.Join(folder, "projects"))
		}, historyShared},
		{"a link through a shared entry that is itself a link", func(t *testing.T, folder, shared string) {
			real := filepath.Join(filepath.Dir(shared), "elsewhere")
			mustMkdir(t, real)
			if err := os.RemoveAll(filepath.Join(shared, "projects")); err != nil {
				t.Fatal(err)
			}
			mustSymlink(t, real, filepath.Join(shared, "projects"))
			mustSymlink(t, filepath.Join(shared, "projects"), filepath.Join(folder, "projects"))
		}, historyShared},
		{"nothing at projects", func(t *testing.T, folder, shared string) {}, historyUnreachable},
		{"a real folder", func(t *testing.T, folder, shared string) {
			mustMkdir(t, filepath.Join(folder, "projects", "-p"))
		}, historyOwn},
		{"a real file", func(t *testing.T, folder, shared string) {
			if err := os.WriteFile(filepath.Join(folder, "projects"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, historyOwn},
		{"a link to a place that is not there", func(t *testing.T, folder, shared string) {
			mustSymlink(t, filepath.Join(folder, "gone"), filepath.Join(folder, "projects"))
		}, historyUnreachable},
		{"a link to another folder that exists", func(t *testing.T, folder, shared string) {
			other := filepath.Join(folder, "other")
			mustMkdir(t, other)
			mustSymlink(t, other, filepath.Join(folder, "projects"))
		}, historyUnreachable},
		{"a link to the right place when that place is gone", func(t *testing.T, folder, shared string) {
			if err := os.RemoveAll(filepath.Join(shared, "projects")); err != nil {
				t.Fatal(err)
			}
			mustSymlink(t, filepath.Join(shared, "projects"), filepath.Join(folder, "projects"))
		}, historyUnreachable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			shared := e.seedShared()
			folder := filepath.Join(e.home, "account")
			mustMkdir(t, folder)

			c.setup(t, folder, shared)

			if got := historyOf(folder); got != c.want {
				t.Errorf("historyOf = %v, want %v", got, c.want)
			}
		})
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}
