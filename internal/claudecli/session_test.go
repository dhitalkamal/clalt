package claudecli

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSessionBuildsTheCommandWithoutRunningIt(t *testing.T) {
	f := newFake(t)
	setAuthEnv(t)
	dir := t.TempDir()
	args := []string{"--model", "opus", "fix the bug"}

	cmd, err := Session(context.Background(), dir, args)

	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if !reflect.DeepEqual(cmd.Args[1:], args) {
		t.Errorf("args = %v, want %v", cmd.Args[1:], args)
	}
	// the caller owns the terminal
	if cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil || cmd.Process != nil {
		t.Error("Session attached stdio or started the command")
	}
	if f.has("argv") {
		t.Error("claude was run")
	}

	if err := cmd.Run(); err != nil {
		t.Fatalf("running the command: %v", err)
	}
	if got := f.argv(); !reflect.DeepEqual(got, args) {
		t.Errorf("claude was called with %v, want %v", got, args)
	}
	f.assertCleanEnv(dir)
}

func TestSessionWithoutArgsPassesNone(t *testing.T) {
	newFake(t)

	cmd, err := Session(context.Background(), t.TempDir(), nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Args) != 1 {
		t.Errorf("args = %v, want only the binary", cmd.Args)
	}
}

// a caller that forgets to look at the error must not be left with a command it
// can start
func TestARefusedSessionHasNoCommandToStart(t *testing.T) {
	newFake(t)

	cmd, err := Session(context.Background(), filepath.Join(t.TempDir(), "not-made"), nil)

	if err == nil || cmd != nil {
		t.Errorf("Session = %v, %v, want no command and an error", cmd, err)
	}
}

func TestSessionWithoutClaudeIsAnError(t *testing.T) {
	t.Setenv("CLALT_CLAUDE_BIN", filepath.Join(t.TempDir(), "no-such-claude"))
	if _, err := Session(context.Background(), t.TempDir(), nil); err == nil {
		t.Error("expected an error with no claude binary")
	}
}

func TestIsAuthEnvVar(t *testing.T) {
	for _, n := range planAuthVars {
		if !IsAuthEnvVar(n) {
			t.Errorf("%s is on the plan list but IsAuthEnvVar says no", n)
		}
	}
	// the config dir is replaced rather than stripped, so it is not one of them
	for _, n := range []string{"", "PATH", "CLAUDE_CONFIG_DIR", "ANTHROPIC_API_KEY_EXTRA", "anthropic_api_key"} {
		// windows env names are case-insensitive, so the lowercase name matches
		if n == "anthropic_api_key" && caseFold {
			continue
		}
		if IsAuthEnvVar(n) {
			t.Errorf("IsAuthEnvVar(%q) = true", n)
		}
	}
}

func TestIsAuthEnvVarIgnoresCaseWhereTheOSDoes(t *testing.T) {
	old := caseFold
	caseFold = true
	t.Cleanup(func() { caseFold = old })

	if !IsAuthEnvVar("Anthropic_Api_Key") {
		t.Error("a differently cased name should match where the OS ignores case")
	}
}
