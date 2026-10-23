package cli

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/config"
)

// the guard is only a guard when it is the claude that a test without a fake
// reaches, and when it refuses to be claude. this test installs no fake.
func TestTheGuardIsWhatClaudeResolvesToWhenNoFakeIsInstalled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the guard is an sh script, which windows cannot run as claude")
	}
	bin, err := claudecli.Binary()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(bin)
	if err != nil || !strings.Contains(string(raw), "a test ran claude without installing a fake") {
		t.Fatalf("CLALT_CLAUDE_BIN does not point at the guard: %s %v", bin, err)
	}
	var exit *exec.ExitError
	if err := exec.Command(bin, "auth", "status").Run(); !errors.As(err, &exit) || exit.ExitCode() != 99 {
		t.Errorf("the guard did not refuse: %v", err)
	}
}

// a test must see none of what the developer has set: not the real home, not an
// XDG folder, not a claude home that CLAUDE_CONFIG_DIR points at, not a claude
// binary, not a login variable. whatever was there before, newEnv puts its own.
func TestNewEnvKeepsWhatTheDeveloperHasSetOutOfATest(t *testing.T) {
	ambient := t.TempDir()
	t.Setenv("HOME", ambient)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(ambient, "xdg"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(ambient, "claude"))
	t.Setenv("CLALT_CLAUDE_BIN", filepath.Join(ambient, "real-claude"))
	t.Setenv("ANTHROPIC_API_KEY", "sk-ambient")
	// somebody else's account, where a test that was not isolated would find it
	plantState(t, `{"accounts":{"intruder":{"email":"i@example.com","addedAt":"2026-01-01T00:00:00Z"}}}`)
	ambientState := config.StatePath()

	e := newEnv(t)

	inside := func(path string) bool {
		return strings.HasPrefix(path, ambient+string(filepath.Separator)) || path == ambient
	}
	for name, path := range map[string]string{
		"HOME": os.Getenv("HOME"), "XDG_CONFIG_HOME": os.Getenv("XDG_CONFIG_HOME"), "the state": config.StatePath(),
		"clalt's folder": config.ClaltDir(), "the shared claude home": config.SharedClaudeDir(), "claude": os.Getenv("CLALT_CLAUDE_BIN"),
	} {
		if inside(path) {
			t.Errorf("%s is %s, inside what the developer had set", name, path)
		}
	}
	if os.Getenv("HOME") != e.home || !strings.HasPrefix(os.Getenv("XDG_CONFIG_HOME"), e.home) {
		t.Errorf("HOME %q and XDG_CONFIG_HOME %q are not inside the folder of the test, %s", os.Getenv("HOME"), os.Getenv("XDG_CONFIG_HOME"), e.home)
	}
	if got := os.Getenv("CLAUDE_CONFIG_DIR"); got != "" {
		t.Errorf("CLAUDE_CONFIG_DIR = %q, want it empty", got)
	}
	if got := os.Getenv("ANTHROPIC_API_KEY"); got != "" {
		t.Errorf("ANTHROPIC_API_KEY = %q, want it empty", got)
	}
	if config.StatePath() == ambientState {
		t.Error("the state is the one the developer's environment pointed at")
	}
	if code := e.run("list", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &rows); err != nil || len(rows) != 0 {
		t.Errorf("the test sees accounts it did not make: %v %s", err, e.out.String())
	}
}
