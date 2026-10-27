package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// what clalt says while it gets a session ready is for the person at the
// terminal, and stdout is claude's: `clalt run work -- -p "..." | jq` has to
// get claude's answer and nothing of ours.
func TestRunLeavesStdoutToClaude(t *testing.T) {
	e := newEnv(t)
	home := e.seedShared()
	mustMkdir(t, filepath.Join(home, "skills", "review"))
	e.signIn("work", "a@example.com")
	// a real folder in the way of one entry of a split folder, a new folder to
	// link, and a variable that is not passed on: one line of each kind
	entry := filepath.Join(e.folder("work"), "skills", "review")
	if err := os.Remove(entry); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, filepath.Join(entry, "mine"))
	mustMkdir(t, filepath.Join(home, "commands"))
	t.Setenv("ANTHROPIC_API_KEY", "not-a-real-key")
	e.claude.set("SESSION_PRINTS", "1")
	e.reset()

	if code := e.run("run", "work", "--", "-p", "hi"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := e.out.String(); got != "hello from claude\n" {
		t.Errorf("stdout = %q, want only what claude printed", got)
	}
	msg := e.errOut.String()
	for _, want := range []string{
		"note: not passing these variables to claude", "ANTHROPIC_API_KEY",
		"synced with the shared claude home: 1 item linked",
		"not linked, left as it is", "skills/review",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("stderr is missing %q:\n%s", want, msg)
		}
	}
}

// the replaced lines are about what clalt did, so they go the same way
func TestRunSaysWhatItReplacedOnStderr(t *testing.T) {
	e := newEnv(t)
	home := e.seedShared()
	mustMkdir(t, filepath.Join(home, "skills", "review"))
	e.signIn("work", "a@example.com")
	skills := filepath.Join(e.folder("work"), "skills")
	if err := os.RemoveAll(skills); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, filepath.Join(home, "skills"), skills)
	e.claude.set("SESSION_PRINTS", "1")
	e.reset()

	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := e.out.String(); got != "hello from claude\n" {
		t.Errorf("stdout = %q, want only what claude printed", got)
	}
	if msg := e.errOut.String(); !strings.Contains(msg, "replaced the link to a whole folder with a folder of links: skills") {
		t.Errorf("stderr = %q", msg)
	}
}

// login keeps its answer on stdout and the notes beside it on stderr
func TestLoginKeepsItsAnswerOnStdoutAndItsNotesOnStderr(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	t.Setenv("ANTHROPIC_API_KEY", "not-a-real-key")
	e.claude.signInAs("a@example.com")

	if code := e.run("login", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	out, msg := e.out.String(), e.errOut.String()
	if !strings.Contains(out, "work is signed in as a@example.com") {
		t.Errorf("stdout = %q", out)
	}
	for _, note := range []string{"note: not passing these variables", "synced with the shared claude home"} {
		if strings.Contains(out, note) || !strings.Contains(msg, note) {
			t.Errorf("%q should be on stderr only.\nstdout: %s\nstderr: %s", note, out, msg)
		}
	}
}
