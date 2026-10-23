package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/registry"
	"github.com/dhitalkamal/clalt/internal/runcmd"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// realRegistry gives the app the real registry and attribution adapters, with
// only the runner and environment faked, and returns the fake runner.
func realRegistry(e *testEnv) *runcmd.Fake {
	e.t.Helper()
	d := realDeps()
	run := runcmd.NewFake()
	d.runner = run
	d.env = func(string) string { return "" }
	e.app.sdeps = d
	return run
}

// addRecord writes a live session record, tied to this test process.
func addRecord(t *testing.T, rec registry.Record) {
	t.Helper()
	rec.WrapperPID = os.Getpid()
	if rec.StartedAt.IsZero() {
		rec.StartedAt = time.Now()
	}
	if err := (registry.Registry{Dir: config.SessionsDir()}).Create(rec); err != nil {
		t.Fatal(err)
	}
}

func TestJumpFindsAKittySessionInTheRegistry(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	run := realRegistry(e)
	addRecord(t, registry.Record{Handle: "aaaa111122223333", Account: e.account("work").Name, Cwd: "/src/a",
		Terminal: registry.Terminal{Kind: "kitty", KittyWindowID: "7", KittyListenOn: "unix:/tmp/k"}})

	if code := e.run("jump", "aaaa"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	want := []runcmd.Call{{Name: "kitten", Args: []string{"@", "--to", "unix:/tmp/k", "focus-window", "--match", "id:7"}}}
	if got := run.Calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestJumpFindsATmuxSessionByClaudeSessionID(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	run := realRegistry(e)
	addRecord(t, registry.Record{Handle: "bbbb111122223333", Account: e.account("work").Name, ClaudeSessionID: "9999ffff-0000",
		Terminal: registry.Terminal{Kind: "tmux", TmuxPane: "%5", TmuxSocket: "/tmp/tmux-1/default"}})

	if code := e.run("jump", "9999ff"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	calls := run.Calls()
	if len(calls) != 3 || calls[0].Name != "tmux" || calls[0].Args[len(calls[0].Args)-1] != "%5" {
		t.Errorf("calls = %v", calls)
	}
}

func TestResumeRefusesAConversationOpenInTheRegistry(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	realRegistry(e)
	cwd := t.TempDir()
	e.seedConversation(convID(7), cwd, "continue me", "2026-10-22T11:00:00Z")
	addRecord(t, registry.Record{Handle: "cccc111122223333", Account: e.account("work").Name, ClaudeSessionID: convID(7)})

	code := e.run("resume", "aaaaaaaa", "--account", "work")

	if code != 1 || e.claude.sessionStarted() || !strings.Contains(e.errOut.String(), "clalt jump cccc111122223333") {
		t.Errorf("exit %d, started %v: %s", code, e.claude.sessionStarted(), e.errOut.String())
	}
}

func TestResumeUsesTheAccountFromTheAttributionLog(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.signIn("home", "b@example.com")
	realRegistry(e)
	e.seedConversation(convID(7), t.TempDir(), "continue me", "2026-10-22T11:00:00Z")
	logAccount := func(ts time.Time, label string) {
		m := usage.SessionAccount{Ts: ts, SessionID: convID(7), Account: e.account(label).Name}
		if err := usage.AppendSessionAccount(config.SessionAccountsPath(), m); err != nil {
			t.Fatal(err)
		}
	}
	// the later line wins, whatever the order in the file
	logAccount(time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC), "home")
	logAccount(time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC), "work")

	if code := e.run("resume", "aaaaaaaa"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if _, env, _ := e.claude.session(); env["CLAUDE_CONFIG_DIR"] != e.folder("home") {
		t.Errorf("ran in %q", env["CLAUDE_CONFIG_DIR"])
	}
}

// bad lines and a zero timestamp do not hide the real mapping.
func TestAttributedAccountSurvivesBadLines(t *testing.T) {
	newEnv(t)
	body := "garbage\n" +
		`{"sessionId":"abc","account":"zero"}` + "\n" +
		strings.Repeat("y", 100*1024) + "\n" +
		`{"ts":"2026-10-01T00:00:00Z","sessionId":"ABC","account":"real"}` + "\n"
	if err := os.MkdirAll(filepath.Dir(config.SessionAccountsPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.SessionAccountsPath(), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := attributedAccount("abc")

	if !ok || got != "real" {
		t.Errorf("got %q %v", got, ok)
	}
}
