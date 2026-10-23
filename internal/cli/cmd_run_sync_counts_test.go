package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// a link that aims elsewhere is re-pointed and counted
func TestRunCountsRepointedLinks(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	link := filepath.Join(e.folder("work"), "settings.json")
	other := filepath.Join(e.home, "elsewhere.json")
	if err := os.WriteFile(other, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, other, link)
	e.reset()

	code := e.run("run", "work")

	if want := "synced with the shared claude home: 1 link repointed"; code != 0 || !strings.Contains(e.errOut.String(), want) {
		t.Errorf("exit %d, want %q in:\n%s", code, want, e.errOut.String())
	}
}

// a link whose source left the shared home is removed and counted
func TestRunCountsDanglingLinksRemoved(t *testing.T) {
	e := newEnv(t)
	home := e.seedShared()
	if err := os.MkdirAll(filepath.Join(home, "commands"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.signIn("work", "a@example.com")
	if err := os.RemoveAll(filepath.Join(home, "commands")); err != nil {
		t.Fatal(err)
	}
	e.reset()

	code := e.run("run", "work")

	if want := "synced with the shared claude home: 1 dangling link removed"; code != 0 || !strings.Contains(e.errOut.String(), want) {
		t.Errorf("exit %d, want %q in:\n%s", code, want, e.errOut.String())
	}
}

// servers that changed or left the shared config are counted
func TestRunCountsUpdatedAndRemovedServers(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	shared := filepath.Join(e.home, ".claude.json")
	for _, step := range []struct{ body, want string }{
		{`{"mcpServers":{"github":{"command":"gh-mcp-2"}}}`, "1 mcp server updated"},
		{`{"mcpServers":{}}`, "1 mcp server removed"},
	} {
		if err := os.WriteFile(shared, []byte(step.body), 0o644); err != nil {
			t.Fatal(err)
		}
		e.reset()

		code := e.run("run", "work")

		if code != 0 || !strings.Contains(e.errOut.String(), step.want) {
			t.Errorf("exit %d, want %q in:\n%s", code, step.want, e.errOut.String())
		}
	}
}

// the name of a server that was replaced is shown as an escape, not as the character
func TestOverwroteServerNamesAreCleaned(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	name := `lin\u001b[31mear`
	own := `{"mcpServers":{"` + name + `":{"command":"mine"}}}`
	if err := os.WriteFile(filepath.Join(e.folder("work"), ".claude.json"), []byte(own), 0o600); err != nil {
		t.Fatal(err)
	}
	shared := `{"mcpServers":{"` + name + `":{"command":"shared"}}}`
	if err := os.WriteFile(filepath.Join(e.home, ".claude.json"), []byte(shared), 0o644); err != nil {
		t.Fatal(err)
	}
	e.reset()

	code := e.run("run", "work")

	out := e.out.String() + e.errOut.String()
	if code != 0 || !strings.Contains(out, "replaced") || hasControl(out) {
		t.Errorf("exit %d, the escape reached the screen: %q", code, out)
	}
}

// the name in a link warning, and the error text that holds it, are cleaned
func TestLinkWarningNamesAreCleaned(t *testing.T) {
	e := newEnv(t)
	home := e.seedShared()
	name := "x\x1b[31mred"
	if err := os.Mkdir(filepath.Join(home, name), 0o755); err != nil {
		t.Skipf("this file system refuses the name: %v", err)
	}
	e.signIn("work", "w@example.com")
	folder := e.folder("work")
	if err := os.Remove(filepath.Join(folder, name)); err != nil {
		t.Fatal(err)
	}
	if !makeReadOnly(t, folder) {
		t.Skip("cannot make a folder read-only here")
	}
	e.reset()

	code := e.run("run", "work")

	msg := e.errOut.String()
	if code != 0 || !strings.Contains(msg, "could not link") || hasControl(msg) {
		t.Errorf("exit %d, stderr %q", code, msg)
	}
}

// a shared home that cannot be read stops the run, the sync is not skipped in silence
func TestRunStopsWhenTheSharedHomeCannotBeRead(t *testing.T) {
	skipWindows(t, "mode 0 does not make a folder unreadable on windows")
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	shared := filepath.Join(e.home, ".claude")
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	t.Cleanup(func() { _ = os.Chmod(shared, 0o755) })
	if err := os.Chmod(shared, 0); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(e.claude.dir, "session.argv"))
	e.reset()

	code := e.run("run", "work")

	if code != 1 || e.claude.sessionStarted() || !strings.Contains(e.errOut.String(), "to the shared claude home") {
		t.Errorf("exit %d, started %v: %s", code, e.claude.sessionStarted(), e.errOut.String())
	}
}

// the message about a missing shared home says what to do
func TestNoSharedHomeMessageSaysWhatToDo(t *testing.T) {
	e := newEnv(t)
	if err := os.Remove(filepath.Join(e.home, ".claude")); err != nil {
		t.Fatal(err)
	}

	code := e.run("login", "work")

	if msg := e.errOut.String(); code != 1 || !strings.Contains(msg, "run claude once") {
		t.Errorf("exit %d: %s", code, msg)
	}
}

// a home that is a link into a volume that is not mounted is a home that is not there
func TestRunSaysTheHomeIsMissingWhenItIsALinkToAnUnmountedVolume(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	shared := filepath.Join(e.home, ".claude")
	if err := os.Rename(shared, shared+".moved"); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, filepath.Join(e.home, "volume-that-is-not-mounted", "claude"), shared)
	e.reset()

	code := e.run("run", "work")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "does not exist") || strings.Contains(msg, "create the shared projects folder") {
		t.Errorf("exit %d: %s", code, msg)
	}
}

// the name of a server of the account's own that the shared one replaces comes
// from a file, and the file can spell the escape out
func TestTheNameOfAReplacedServerIsPrintedEscaped(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	name := "x\\u001b[31mred"
	servers := func(command string) []byte {
		return []byte(`{"mcpServers":{"github":{"command":"gh-mcp"},"` + name + `":{"command":"` + command + `"}}}`)
	}
	if err := os.WriteFile(filepath.Join(e.folder("work"), ".claude.json"), servers("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.home, ".claude.json"), servers("shared"), 0o600); err != nil {
		t.Fatal(err)
	}

	e.reset()
	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	msg := e.errOut.String()
	if !strings.Contains(msg, "replaced the account's own mcp server") || !strings.Contains(msg, `x\x1b[31mred`) || hasControl(msg) {
		t.Errorf("the replaced server is not named, or not cleaned: %q", msg)
	}
}
