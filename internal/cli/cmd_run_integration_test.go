package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dhitalkamal/clalt/internal/account"
)

// TestRunInjectsTokenEndToEnd drives the real cmdRun path with a stub claude
// binary and proves the account's token reaches the child process through
// CLAUDE_CODE_OAUTH_TOKEN on macOS (the keychain-free launch), end to end.
func TestRunInjectsTokenEndToEnd(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS injects the token via CLAUDE_CODE_OAUTH_TOKEN; other OSes use profile files")
	}
	app, _, dir := newTestApp(t)
	t.Setenv("XDG_CONFIG_HOME", dir)

	// seed an account in the vault (no OAuth needed)
	if err := app.Store.Save("work", liveCred("tok-e2e"), account.Meta{}); err != nil {
		t.Fatal(err)
	}

	// a stub "claude" that records the token it was handed
	dump := filepath.Join(dir, "seen-token.txt")
	stub := filepath.Join(dir, "claude-stub.sh")
	script := "#!/bin/sh\nprintf '%s' \"$CLAUDE_CODE_OAUTH_TOKEN\" > \"" + dump + "\"\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLALT_CLAUDE_BIN", stub)

	if err := app.cmdRun([]string{"work"}); err != nil {
		t.Fatalf("cmdRun: %v", err)
	}

	got, err := os.ReadFile(dump)
	if err != nil {
		t.Fatalf("stub claude did not run or wrote nothing: %v", err)
	}
	if string(got) != "tok-e2e" {
		t.Errorf("child claude saw CLAUDE_CODE_OAUTH_TOKEN=%q, want tok-e2e", string(got))
	}
}
