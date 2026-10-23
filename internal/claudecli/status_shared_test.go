package claudecli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// the shared home may be asked who is signed in (a read), and nothing else
func TestStatusOfSharedAsksClaudeAboutTheSharedHome(t *testing.T) {
	f := newFake(t)
	f.say("status", 1, signedOutJSON)
	home := t.TempDir()
	t.Setenv("HOME", home)
	shared := filepath.Join(home, ".claude")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}

	st, err := StatusOfShared(context.Background(), shared)

	if err != nil || st.LoggedIn {
		t.Fatalf("StatusOfShared = %+v, %v", st, err)
	}
	if !f.has("argv") {
		t.Error("claude was not asked")
	}
}

func TestStatusOfSharedStillRefusesAFolderThatIsNotThere(t *testing.T) {
	f := newFake(t)
	f.say("status", 1, signedOutJSON)
	_, err := StatusOfShared(context.Background(), filepath.Join(t.TempDir(), "not-made"))
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("err = %v", err)
	}
	if f.has("argv") {
		t.Error("claude was run for a missing folder")
	}
}

// the default home is what plain claude sees with no CLAUDE_CONFIG_DIR, and on
// a mac its keychain entry is named differently when that variable is set
func TestStatusOfSharedLeavesTheConfigDirUnsetForTheDefaultHome(t *testing.T) {
	f := newFake(t)
	f.say("status", 1, signedOutJSON)
	home := t.TempDir()
	t.Setenv("HOME", home)
	shared := filepath.Join(home, ".claude")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANTHROPIC_API_KEY", "secret-key")

	if _, err := StatusOfShared(context.Background(), shared); err != nil {
		t.Fatal(err)
	}

	env := f.envOf("status")
	if v, set := env["CLAUDE_CONFIG_DIR"]; set && v != "" {
		t.Errorf("CLAUDE_CONFIG_DIR was set for the default home")
	}
	if _, set := env["ANTHROPIC_API_KEY"]; set {
		t.Error("an auth variable was passed on")
	}
}

func TestStatusOfSharedSetsTheConfigDirForACustomHome(t *testing.T) {
	f := newFake(t)
	f.say("status", 1, signedOutJSON)
	custom := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", custom)

	if _, err := StatusOfShared(context.Background(), custom); err != nil {
		t.Fatal(err)
	}

	if got := f.envOf("status")["CLAUDE_CONFIG_DIR"]; got != custom {
		t.Errorf("CLAUDE_CONFIG_DIR = %q, want %q", got, custom)
	}
}
