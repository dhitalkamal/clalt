package claudecli

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoginBuildsTheCommandWithoutRunningIt(t *testing.T) {
	f := newFake(t)
	setAuthEnv(t)
	dir := t.TempDir()

	cmd, err := Login(context.Background(), dir, "")

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !reflect.DeepEqual(cmd.Args[1:], []string{"auth", "login"}) {
		t.Errorf("args = %v", cmd.Args[1:])
	}
	// the caller owns the terminal: the dashboard hands this to tea.ExecProcess
	if cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil || cmd.Process != nil {
		t.Error("Login attached stdio or started the command")
	}
	if f.ran("auth") != 0 {
		t.Error("claude auth login was run")
	}
	var dirs int
	for _, kv := range cmd.Env {
		name, _, _ := strings.Cut(kv, "=")
		for _, n := range authEnvVars {
			if name == n {
				t.Errorf("%s is in the command's environment", n)
			}
		}
		if kv == "CLAUDE_CONFIG_DIR="+dir {
			dirs++
		}
	}
	if dirs != 1 {
		t.Errorf("CLAUDE_CONFIG_DIR=%s appears %d times, want once", dir, dirs)
	}
}

func TestLoginPassesTheEmailWhenGiven(t *testing.T) {
	cases := map[string][]string{
		"":                  {"auth", "login"},
		"robin@example.com": {"auth", "login", "--email", "robin@example.com"},
	}
	f := newFake(t)
	setAuthEnv(t)
	for email, want := range cases {
		dir := t.TempDir()
		cmd, err := Login(context.Background(), dir, email)
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Run(); err != nil {
			t.Fatalf("running the command: %v", err)
		}
		if got := f.argv(); !reflect.DeepEqual(got, want) {
			t.Errorf("email %q: claude was called with %v, want %v", email, got, want)
		}
		f.assertCleanEnv(dir)
	}
}

func TestLoginRefusesAnEmailThatLooksLikeAFlag(t *testing.T) {
	newFake(t)
	if _, err := Login(context.Background(), t.TempDir(), "--dangerously-skip-permissions"); err == nil {
		t.Error("Login passed an email that claude would read as a flag")
	}
}

func TestLoginWithoutClaudeIsAnError(t *testing.T) {
	skipOnWindows(t)
	isolateHome(t, t.TempDir())
	t.Setenv("CLALT_CLAUDE_BIN", filepath.Join(t.TempDir(), "no-such-claude"))
	if _, err := Login(context.Background(), t.TempDir(), ""); err == nil {
		t.Error("expected an error with no claude binary")
	}
}

func TestLogoutFailureIsAnErrorWithTheFirstStderrLine(t *testing.T) {
	f := newFake(t)
	f.say("logout", 1, "")
	f.complain("logout", "cannot reach the keychain\nsecond line")

	err := Logout(context.Background(), t.TempDir())

	if err == nil {
		t.Fatal("a non-zero exit must be an error")
	}
	if !strings.Contains(err.Error(), "cannot reach the keychain") || strings.Contains(err.Error(), "second line") {
		t.Errorf("error = %q", err)
	}
}

func TestLogoutWithoutClaudeIsAnError(t *testing.T) {
	skipOnWindows(t)
	isolateHome(t, t.TempDir())
	t.Setenv("CLALT_CLAUDE_BIN", filepath.Join(t.TempDir(), "no-such-claude"))
	if err := Logout(context.Background(), t.TempDir()); err == nil {
		t.Error("expected an error with no claude binary")
	}
}
