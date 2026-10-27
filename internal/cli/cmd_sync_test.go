package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// the shared claude home may only be unmounted, so clalt never makes it. without
// it there is nothing to link to, and login and run say so.
func TestLoginAndRunStopWhenThereIsNoSharedHome(t *testing.T) {
	e := newEnv(t)
	shared := filepath.Join(e.home, ".claude")
	if err := os.Remove(shared); err != nil {
		t.Fatal(err)
	}

	if code := e.run("login", "work"); code != 1 {
		t.Fatalf("login: exit %d: %s", code, e.errOut.String())
	}
	msg := e.errOut.String()
	for _, want := range []string{shared, "does not exist", "claude"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the message should say %q:\n%s", want, msg)
		}
	}
	if exists(shared) {
		t.Error("clalt made the shared claude home")
	}
	noClaudeCalls(t, e)

	// an account that worked, and then the home goes away
	e.seedShared()
	e.signIn("home", "h@example.com")
	if err := os.RemoveAll(shared); err != nil {
		t.Fatal(err)
	}
	e.reset()
	if code := e.run("run", "home"); code != 1 || !strings.Contains(e.errOut.String(), shared) {
		t.Errorf("run: exit %d: %s", code, e.errOut.String())
	}
	if e.claude.sessionStarted() {
		t.Error("a session started without a shared home")
	}
	if link, err := os.Readlink(filepath.Join(e.folder("home"), "projects")); err != nil || link == "" {
		t.Errorf("the account lost its projects link: %q, %v", link, err)
	}
}

// a machine that ran claude but never saved a conversation has a home and no
// projects folder, and without one the history would stay with the account.
func TestSyncMakesTheSharedProjectsFolderInAHomeThatExists(t *testing.T) {
	e := newEnv(t) // a home with nothing in it
	shared := filepath.Join(e.home, ".claude")

	e.signIn("work", "a@example.com")

	link, err := os.Readlink(filepath.Join(e.folder("work"), "projects"))
	if err != nil || link != filepath.Join(shared, "projects") {
		t.Errorf("projects links to %q (%v): history would be per account", link, err)
	}
}

func TestSyncNeverMakesAFolderInTheWorkingDirectoryWhenTheHomeIsUnknown(t *testing.T) {
	e := newEnv(t)
	cwd := t.TempDir()
	t.Chdir(cwd)
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	if _, err := e.app.syncAccount(filepath.Join(e.home, "folder")); err == nil || !strings.Contains(err.Error(), "HOME") {
		t.Errorf("syncing without a known home should say so, got %v", err)
	}
	if entries, _ := os.ReadDir(cwd); len(entries) != 0 {
		t.Errorf("the working directory now holds %d entries", len(entries))
	}
}

func TestRunNamesKeptItemsWithTheirReasonAndStillRuns(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	settings := filepath.Join(e.folder("work"), "settings.json")
	if err := os.Remove(settings); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	code := e.run("run", "work")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	out := e.errOut.String()
	for _, want := range []string{"settings.json", "a real file is in the way", "clalt doctor"} {
		if !strings.Contains(out, want) {
			t.Errorf("the kept file is not explained, missing %q:\n%s", want, out)
		}
	}
	if !e.claude.sessionStarted() {
		t.Error("a kept settings file should not stop the session")
	}
}

// the entries of a split folder are linked one by one, and are named as such.
func TestRunNamesAKeptEntryOfASplitFolder(t *testing.T) {
	e := newEnv(t)
	home := e.seedShared()
	if err := os.MkdirAll(filepath.Join(home, "skills", "review"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.signIn("work", "a@example.com")
	entry := filepath.Join(e.folder("work"), "skills", "review")
	if err := os.Remove(entry); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(entry, "mine"), 0o755); err != nil {
		t.Fatal(err)
	}

	e.reset()
	code := e.run("run", "work")

	if out := e.errOut.String(); code != 0 || !strings.Contains(out, "skills/review") || !strings.Contains(out, "a real folder is in the way") {
		t.Errorf("exit %d, the kept entry is not named with its reason:\n%s", code, out)
	}
}

func TestRunRefusesWhenProjectsIsNotLinkedAndSaysWhy(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	projects := filepath.Join(e.folder("work"), "projects")
	if err := os.Remove(projects); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projects, []byte("not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}

	code := e.run("run", "work")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "history would not be shared") || !strings.Contains(msg, "a real file is in the way") {
		t.Errorf("exit %d: %s", code, msg)
	}
	if e.claude.sessionStarted() {
		t.Error("claude started with a history that is not shared")
	}
}

// a shared server of the same name wins over one the account added itself, and
// the user is told, by name only.
func TestRunSaysWhenASharedServerReplacedTheAccountsOwn(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	own := `{"mcpServers":{"github":{"command":"gh-mcp"},"linear":{"command":"my-linear-secret"}}}`
	if err := os.WriteFile(filepath.Join(e.folder("work"), ".claude.json"), []byte(own), 0o600); err != nil {
		t.Fatal(err)
	}
	shared := `{"mcpServers":{"github":{"command":"gh-mcp"},"linear":{"command":"shared-linear-secret"}}}`
	if err := os.WriteFile(filepath.Join(e.home, ".claude.json"), []byte(shared), 0o644); err != nil {
		t.Fatal(err)
	}

	e.reset()
	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	out := e.errOut.String()
	if !strings.Contains(out, "replaced") || !strings.Contains(out, "linear") {
		t.Errorf("the replaced server is not named:\n%s", out)
	}
	if strings.Contains(e.out.String()+out, "secret") {
		t.Error("a server definition leaked into the output")
	}
}

// a shared config that is gone while servers were mirrored from it is more
// likely an unmounted home than a decision to drop every server, so the sync
// stops and the account keeps what it has.
func TestASharedConfigThatIsGoneIsAWarningAndTheAccountKeepsItsServers(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	if err := os.Remove(filepath.Join(e.home, ".claude.json")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(e.folder("work"), ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}

	e.reset()
	code := e.run("run", "work")

	if code != 0 || !strings.Contains(e.errOut.String(), "warning: could not sync mcp servers") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if after, _ := os.ReadFile(filepath.Join(e.folder("work"), ".claude.json")); string(after) != string(before) {
		t.Errorf("the account's servers were changed:\n%s\nthen\n%s", before, after)
	}
	_, rep := doctor(t, e)
	rep.want(t, "work", "mcp", "warn", "could not check the mcp servers")
}

// skills and plugins used to be linked whole. the link is replaced by a real
// folder with links inside, which is not the same as aiming a link elsewhere,
// and the line says so.
func TestRunSaysWhenALinkToAWholeFolderWasReplaced(t *testing.T) {
	e := newEnv(t)
	home := e.seedShared()
	if err := os.MkdirAll(filepath.Join(home, "skills", "review"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.signIn("work", "a@example.com")
	skills := filepath.Join(e.folder("work"), "skills")
	if err := os.RemoveAll(skills); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "skills"), skills); err != nil {
		t.Fatal(err)
	}

	e.reset()
	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	out := e.errOut.String()
	if !strings.Contains(out, "replaced the link to a whole folder with a folder of links: skills") || strings.Contains(out, "repointed") {
		t.Errorf("the replaced link is not described as such:\n%s", out)
	}
	if fi, err := os.Lstat(skills); err != nil || !fi.IsDir() {
		t.Errorf("skills is not a real folder now: %v", err)
	}
}
