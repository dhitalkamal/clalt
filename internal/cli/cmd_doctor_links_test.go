package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorNamesKeptEntriesWithTheirReason(t *testing.T) {
	e := newEnv(t)
	home := e.seedShared()
	if err := os.MkdirAll(filepath.Join(home, "skills", "review"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.signIn("work", "w@example.com")
	settings := filepath.Join(e.folder("work"), "settings.json")
	entry := filepath.Join(e.folder("work"), "skills", "review")
	for _, path := range []string{settings, entry} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(settings, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(entry, "mine"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, e.folder("work"))

	code, rep := doctor(t, e)

	rep.want(t, "work", "links", "warn", "settings.json (a real file is in the way)")
	rep.want(t, "work", "links", "warn", "skills/review (a real folder is in the way)")
	if code != 0 {
		t.Errorf("a kept entry is a warning, exit %d", code)
	}
	if after := snapshot(t, e.folder("work")); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Error("doctor changed the account folder")
	}
}

func TestDoctorSaysWhenThereIsNoSharedHome(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	shared := filepath.Join(e.home, ".claude")
	if err := os.RemoveAll(shared); err != nil {
		t.Fatal(err)
	}

	code, rep := doctor(t, e)

	rep.want(t, "work", "links", "warn", shared)
	rep.want(t, "work", "links", "warn", "does not exist")
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	if exists(shared) {
		t.Error("doctor made the shared claude home")
	}
}

// claude.ai sessions download an account's own skills and plugins into synced
// folders, and its marketplaces into a json file. they stay per account by
// design, so there is nothing for doctor to say about them.
func TestDoctorSaysNothingAboutTheFoldersThatStayPerAccount(t *testing.T) {
	e := newEnv(t)
	home := e.seedShared()
	for _, dir := range []string{"skills/review", "plugins/tools"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	e.signIn("work", "w@example.com")
	folder := e.folder("work")
	for path, body := range map[string]string{
		filepath.Join(folder, "skills", "synced", "downloaded", "SKILL.md"):  "x",
		filepath.Join(folder, "plugins", "synced", "p", "plugin.json"):       "{}",
		filepath.Join(folder, "plugins", "known_marketplaces_claudeai.json"): "{}",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	code, rep := doctor(t, e)

	rep.want(t, "work", "links", "ok", "")
	if code != 0 || rep.Warned != 0 || strings.Contains(e.out.String(), "synced") || strings.Contains(e.out.String(), "marketplaces") {
		t.Errorf("exit %d, %d warned:\n%s", code, rep.Warned, e.out.String())
	}
}

// doctor says what run would refuse for. a shared history on a volume that is
// not mounted is something to do (mount it), so it is a warning, and it is not
// a folder with something in it to move.
func TestDoctorWarnsWhenTheSharedHistoryIsNotReachable(t *testing.T) {
	cases := map[string]func(e *testEnv){
		"an account that had a link": func(e *testEnv) {
			e.signIn("work", "w@example.com")
			unmountHistory(t, e)
		},
		"an account made while it was unmounted": func(e *testEnv) {
			unmountHistory(t, e)
			e.signIn("work", "w@example.com")
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.seedShared()
			setup(e)
			before := snapshot(t, e.folder("work"))

			code, rep := doctor(t, e)

			rep.want(t, "work", "links", "warn", "not reachable")
			rep.want(t, "work", "links", "warn", "unmounted")
			if c, _ := rep.find("work", "links"); strings.Contains(c.Detail, "move what is in it") {
				t.Errorf("there is nothing in the account to move: %s", c.Detail)
			}
			if code != 0 {
				t.Errorf("exit %d: something to mount is a warning", code)
			}
			if after := snapshot(t, e.folder("work")); strings.Join(after, "\n") != strings.Join(before, "\n") {
				t.Error("doctor changed the account folder")
			}
		})
	}
}

// the warning joins what else is out of sync instead of hiding it.
func TestDoctorKeepsTheOtherFindingsNextToTheHistoryOne(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	unmountHistory(t, e)
	if err := os.Remove(filepath.Join(e.folder("work"), "settings.json")); err != nil {
		t.Fatal(err)
	}

	_, rep := doctor(t, e)

	rep.want(t, "work", "links", "warn", "settings.json")
	rep.want(t, "work", "links", "warn", "not reachable")
}

// a projects link that is simply missing, with the shared folder there, is
// something run puts right, and is not reported as an unreachable history.
func TestDoctorListsAMissingProjectsLinkAsSomethingRunFixes(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	if err := os.Remove(filepath.Join(e.folder("work"), "projects")); err != nil {
		t.Fatal(err)
	}

	code, rep := doctor(t, e)

	rep.want(t, "work", "links", "warn", "1 link to fix, 'clalt run work' does it (projects)")
	if c, _ := rep.find("work", "links"); strings.Contains(c.Detail, "not reachable") || code != 0 {
		t.Errorf("exit %d: %s", code, c.Detail)
	}
}
