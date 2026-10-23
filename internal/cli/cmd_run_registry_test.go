package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/registry"
)

// the record the session saw while it ran, parsed
func runningRecord(t *testing.T, e *testEnv) registry.Record {
	t.Helper()
	var rec registry.Record
	if err := json.Unmarshal([]byte(e.claude.sessionRecords()), &rec); err != nil {
		t.Fatalf("record while claude ran: %v: %q", err, e.claude.sessionRecords())
	}
	return rec
}

func TestRunWritesTheRecordBeforeClaudeStartsAndPassesTheHandle(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	t.Setenv("KITTY_WINDOW_ID", "9")
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("TERM_PROGRAM", "")

	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	rec := runningRecord(t, e)
	_, env, _ := e.claude.session()
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(rec.Handle) || env["CLALT_SESSION"] != rec.Handle {
		t.Errorf("handle %q, CLALT_SESSION %q", rec.Handle, env["CLALT_SESSION"])
	}
	cwd, _ := os.Getwd()
	if rec.Account != e.account("work").Name || rec.Folder != e.folder("work") || rec.Cwd != cwd {
		t.Errorf("record %+v", rec)
	}
	if rec.WrapperPID != os.Getpid() {
		t.Errorf("wrapperPid = %d, want %d", rec.WrapperPID, os.Getpid())
	}
	if rec.Terminal.Kind != "kitty" || rec.Terminal.KittyWindowID != "9" {
		t.Errorf("terminal %+v", rec.Terminal)
	}
	if rec.PID == 0 {
		t.Error("claude's pid was never recorded")
	}
}

func TestRunReplacesAnInheritedHandle(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	t.Setenv("CLALT_SESSION", "0000000000000000")

	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if _, env, _ := e.claude.session(); env["CLALT_SESSION"] == "0000000000000000" || env["CLALT_SESSION"] == "" {
		t.Errorf("CLALT_SESSION = %q", env["CLALT_SESSION"])
	}
}

func TestRunStartsWhenTheRecordCannotBeWritten(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	// a file where the sessions folder belongs
	if err := os.WriteFile(filepath.Join(e.home, ".config", "clalt", "sessions"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	code := e.run("run", "work")

	if code != 0 || !e.claude.sessionStarted() || !strings.Contains(e.errOut.String(), "could not record the session") {
		t.Errorf("exit %d started %v: %s", code, e.claude.sessionStarted(), e.errOut.String())
	}
}
