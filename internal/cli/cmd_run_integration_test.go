package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// TestRunRejectsPathTraversalName proves `clalt run` refuses an account name
// that would escape config.ProfileDir (e.g. from a legacy pre-validation
// account, or a name typed straight on the command line) before ever touching
// the filesystem for it: no profile dir gets created anywhere, including
// outside the intended profiles root.
func TestRunRejectsPathTraversalName(t *testing.T) {
	app, _, dir := newTestApp(t)
	t.Setenv("XDG_CONFIG_HOME", dir)

	err := app.cmdRun([]string{"../../../../tmp/evil"})
	if err == nil {
		t.Fatal("cmdRun with a path-traversal name should error")
	}
	if !strings.Contains(err.Error(), "only letters, digits") {
		t.Errorf("error should be the name-validation error, got: %v", err)
	}
	// the profiles root itself must never even get created for a rejected name
	if _, statErr := os.Stat(filepath.Join(dir, "clalt", "profiles")); statErr == nil {
		t.Error("cmdRun should not have created the profiles dir for a rejected name")
	}
}

// TestRunRefusesInvalidLegacyNameWithRenameHint proves that when an existing
// (pre-validation) account has an invalid name, `run` refuses it rather than
// staging secrets into an unsafe path, and tells the user to rename it.
func TestRunRefusesInvalidLegacyNameWithRenameHint(t *testing.T) {
	app, _, dir := newTestApp(t)
	t.Setenv("XDG_CONFIG_HOME", dir)

	legacy := "weird name!"
	if err := app.KC.Set(account.KeychainService, legacy, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	err := app.cmdRun([]string{legacy})
	if err == nil {
		t.Fatal("cmdRun with an invalid legacy name should error")
	}
	if !strings.Contains(err.Error(), "clalt rename") {
		t.Errorf("error should point the user at `clalt rename`, got: %v", err)
	}
}
