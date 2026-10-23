package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// extendFake adds lines to the session part of the fake, before its final exit.
func extendFake(t *testing.T, e *testEnv, extra string) {
	t.Helper()
	bin := filepath.Join(e.claude.dir, "claude")
	raw, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	marker := "if [ \"$FAKE_CLAUDE_EXIT\" = \"term\" ]; then kill -TERM $$; fi\n"
	s := strings.Replace(string(raw), marker, extra+"\n"+marker, 1)
	if s == string(raw) {
		t.Fatal("marker not found in the fake")
	}
	if err := os.WriteFile(bin, []byte(s), 0o755); err != nil {
		t.Fatal(err)
	}
}

func sessionRecordFiles(e *testEnv) []string {
	left, _ := filepath.Glob(filepath.Join(e.home, ".config", "clalt", "sessions", "*.json"))
	return left
}

// a state file that cannot be read is said so, not "no account called"
func TestRunSaysWhenStateCannotBeRead(t *testing.T) {
	e := newEnv(t)
	plantState(t, "not json")

	code := e.run("run", "work")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "state.json") || strings.Contains(msg, "no account called") {
		t.Errorf("exit %d: %s", code, msg)
	}
}

// a refused run of a removed account does nothing to its folder
func TestRunOfARemovedAccountTouchesNothing(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("gone", "g@example.com")
	if code := e.run("remove", "gone", "--force"); code != 0 {
		t.Fatalf("remove exited %d: %s", code, e.errOut.String())
	}
	link := filepath.Join(e.folder("gone"), "projects")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	statusBefore := len(e.claude.calls("status"))
	e.reset()

	code := e.run("run", "gone")

	if code != 1 || !strings.Contains(e.errOut.String(), "was removed") {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if exists(link) {
		t.Error("a refused run synced the folder of a removed account")
	}
	if n := len(e.claude.calls("status")); n != statusBefore {
		t.Errorf("claude was asked %d more times about a removed account", n-statusBefore)
	}
}

// the variables that were taken out of the status call are named even when the run is refused
func TestRefusedRunStillNamesTheStrippedVariables(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.signOut(e.folder("work"))
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-secret")
	e.reset()

	code := e.run("run", "work")

	out := e.out.String() + e.errOut.String()
	if code != 1 || !strings.Contains(out, "ANTHROPIC_API_KEY") || strings.Contains(out, "sk-test-secret") {
		t.Errorf("exit %d:\n%s", code, out)
	}
}

// a failure to record the use is a warning, the session still starts
func TestRunStartsWhenTheUseCannotBeRecorded(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	if code := e.run("run", "work"); code != 0 { // makes the sessions folder
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	cfg := filepath.Join(e.home, ".config", "clalt")
	if !makeReadOnly(t, cfg) {
		t.Skip("cannot make a folder read-only here")
	}
	os.Remove(filepath.Join(e.claude.dir, "session.argv"))
	e.reset()

	code := e.run("run", "work")

	if code != 0 || !e.claude.sessionStarted() || !strings.Contains(e.errOut.String(), "could not record the use of work") {
		t.Errorf("exit %d, started %v: %s", code, e.claude.sessionStarted(), e.errOut.String())
	}
}

// the use is recorded when claude starts, not when it ends
func TestLastUseIsRecordedBeforeClaudeRuns(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	extendFake(t, e, `cp "$XDG_CONFIG_HOME/clalt/state.json" "$dir/session.state"`)

	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	raw, err := os.ReadFile(filepath.Join(e.claude.dir, "session.state"))
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		Accounts map[string]struct {
			LastUsedAt time.Time `json:"lastUsedAt"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if got := st.Accounts[e.account("work").Name].LastUsedAt; got.IsZero() {
		t.Errorf("while claude ran, lastUsedAt was not set yet:\n%s", raw)
	}
}

// renaming never breaks the record of the last use
func TestLastUseOfARenamedAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	if err := e.app.Store.Relabel(e.account("work").Name, "office"); err != nil {
		t.Fatal(err)
	}
	e.reset()

	code := e.run("run", "office")

	if code != 0 || strings.Contains(e.errOut.String(), "could not record") || e.account("office").Meta.LastUsedAt.IsZero() {
		t.Errorf("exit %d, lastUsedAt %v: %s", code, e.account("office").Meta.LastUsedAt, e.errOut.String())
	}
}

// every signal gives 128 plus its number, not only TERM
func TestExitCodeOfOtherSignals(t *testing.T) {
	for name, want := range map[string]int{"KILL": 137, "HUP": 129} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "a@example.com")
			extendFake(t, e, `if [ -n "$FAKE_CLAUDE_SIGNAL" ]; then kill -"$FAKE_CLAUDE_SIGNAL" $$; fi`)
			t.Setenv("FAKE_CLAUDE_SIGNAL", name)

			if code := e.run("run", "work"); code != want {
				t.Errorf("exit code = %d, want %d", code, want)
			}
		})
	}
}

// the record goes away when claude fails too
func TestSessionRecordIsRemovedWhenClaudeFails(t *testing.T) {
	for _, exit := range []string{"7", "term"} {
		t.Run(exit, func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "a@example.com")
			e.claude.set("EXIT", exit)

			e.run("run", "work")

			if left := sessionRecordFiles(e); len(left) != 0 {
				t.Errorf("records left behind after exit %s: %v", exit, left)
			}
		})
	}
}

// the record has the time of the start and the pid of claude
func TestSessionRecordHasTheStartAndClaudesPid(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.at(theClock)
	extendFake(t, e, `echo $$ > "$dir/session.pid"`)

	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	pid, err := os.ReadFile(filepath.Join(e.claude.dir, "session.pid"))
	if err != nil {
		t.Fatal(err)
	}
	rec := e.claude.sessionRecords()
	if !strings.Contains(rec, `"pid":`+strings.TrimSpace(string(pid))+",") {
		t.Errorf("the record is not about claude's pid %s: %s", pid, rec)
	}
	if !strings.Contains(rec, `"startedAt":"2026-10-22T12:00:00Z"`) {
		t.Errorf("the record has no start time: %s", rec)
	}
}

// claude's stderr is the caller's stderr
func TestSessionStderrReachesStderr(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	extendFake(t, e, `echo "problem on stderr" >&2`)

	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if !strings.Contains(e.errOut.String(), "problem on stderr") || strings.Contains(e.out.String(), "problem on stderr") {
		t.Errorf("stdout %q, stderr %q", e.out.String(), e.errOut.String())
	}
}
