package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
)

// hostile is text that would move a terminal around or forge a line of the
// output if it were printed as it is.
func hostile() string { return "evil\x1b]0;PWNED\x07\nerror: forged line" }

// forgedIn is true when out holds an escape sequence, or a line that someone
// else wrote as if it were clalt's.
func forgedIn(out string) bool {
	return hasControl(out) || strings.Contains(out, "\nerror: forged line")
}

// CLAUDE_CONFIG_DIR is whatever the shell has. in the text it is cleaned, and
// the JSON carries the path as it is, since the encoder escapes it already.
func TestWhoamiCleansWhatTheEnvironmentHolds(t *testing.T) {
	for name, dir := range map[string]func() string{
		"unknown account": func() string { return filepath.Join(config.AccountsDir(), hostile()) },
		"old profile":     func() string { return filepath.Join(config.ProfilesDir(), hostile()) },
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			t.Setenv("CLAUDE_CONFIG_DIR", dir())

			if code := e.run("whoami"); code != 0 {
				t.Fatalf("exit %d: %s", code, e.errOut.String())
			}
			if out := e.out.String(); forgedIn(out) || !strings.Contains(out, "evil") {
				t.Errorf("stdout = %q", out)
			}

			e.reset()
			if code := e.run("whoami", "--json"); code != 0 {
				t.Fatalf("exit %d: %s", code, e.errOut.String())
			}
			var got map[string]string
			if err := json.Unmarshal(e.out.Bytes(), &got); err != nil {
				t.Fatalf("not json: %v\n%s", err, e.out.String())
			}
			if got["folder"] != dir() {
				t.Errorf("folder = %q, want the path as it is, %q", got["folder"], dir())
			}
		})
	}
}

// the folder of an account is under XDG_CONFIG_HOME, which can be anywhere
func TestWhoamiCleansTheFolderOfTheAccount(t *testing.T) {
	e := newEnv(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(e.home, hostile()))
	plantState(t, `{"accounts":{"work":{"email":"w@example.com","addedAt":"2026-08-01T10:00:00Z"}}}`)
	folder, err := config.AccountDirFor("work")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", folder)
	// the store keeps the path it was made with, so the app is made again
	app := NewApp()
	app.Out, app.Err, app.In = e.out, e.errOut, strings.NewReader("")
	e.app = app

	if code := e.run("whoami"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if out := e.out.String(); !strings.Contains(out, "folder:") || forgedIn(out) {
		t.Errorf("stdout = %q", out)
	}
}

// the shared claude home can be anywhere the user pointed CLAUDE_CONFIG_DIR at
func TestTheHistoryWarningCleansThePathItNames(t *testing.T) {
	e := newEnv(t)
	shared := filepath.Join(e.home, hostile())
	mustMkdir(t, shared)
	mustSymlink(t, "/nonexistent-volume/projects", filepath.Join(shared, "projects"))
	t.Setenv("CLAUDE_CONFIG_DIR", shared)
	e.claude.signInAs("a@example.com")

	e.run("login", "work")

	if msg := e.errOut.String(); !strings.Contains(msg, "does not reach") || forgedIn(msg) {
		t.Errorf("stderr = %q", msg)
	}
}

// a session file is a file like any other: its folder and an account id that no
// account has any more are shown cleaned, and kept as they are in the JSON
func TestSessionsCleansTheWorkingDirectoryAndTheLabel(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	for _, s := range []profile.Session{
		{Account: "work", PID: os.Getpid(), Cwd: "/tmp/" + hostile(), StartedAt: time.Now()},
		{Account: hostile(), PID: os.Getppid(), Cwd: "/tmp/x", StartedAt: time.Now()},
	} {
		if err := profile.WriteSession(config.SessionsDir(), s); err != nil {
			t.Fatal(err)
		}
	}

	if code := e.run("sessions"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	// both sessions are listed, which a dead one would not be
	if out := e.out.String(); strings.Count(out, "evil") != 2 || forgedIn(out) {
		t.Errorf("stdout = %q", out)
	}
}

// writeTranscriptOf is a reply of 300 tokens from the model, in the project
// folder of that name. model is written into the JSON line as it is.
func (e *testEnv) writeTranscriptOf(project, model string) {
	e.t.Helper()
	dir := filepath.Join(config.ClaudeProjectsDir(), project)
	mustMkdir(e.t, dir)
	line := strings.Replace(replyLine, "claude-sonnet-5", model, 1)
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(line), 0o600); err != nil {
		e.t.Fatal(err)
	}
}

// the model id and the project come out of the transcripts, which anything can
// write to
func TestUsageCleansTheModelAndTheProjectOfATranscript(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.writeSwitchLog(switchToWork)
	e.writeTranscriptOf(hostile(), `claude\u001b]0;PWNED\u0007\nerror: forged line`)

	e.reset()
	if code := e.run("usage"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	out := e.out.String()
	if forgedIn(out) || !strings.Contains(out, "claude") || !strings.Contains(out, "evil") {
		t.Errorf("stdout = %q", out)
	}

	// the JSON keeps both as they are
	row := rowNamed(usageRows(t, e), "work")
	models, _ := row["models"].([]any)
	if len(models) != 1 || models[0].(map[string]any)["model"] != "claude\x1b]0;PWNED\x07\nerror: forged line" {
		t.Errorf("models = %v", row["models"])
	}
}
