package claudecli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLogoutChecksThatTheFolderIsSignedOutAfterwards(t *testing.T) {
	f := newFake(t)
	f.say("logout", 0, "Signed out\n")
	f.say("status", 1, signedOutJSON)
	setAuthEnv(t)
	dir := t.TempDir()

	if err := Logout(context.Background(), dir); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	calls := f.calls()
	if len(calls) < 2 || calls[len(calls)-2] != "auth logout" || calls[len(calls)-1] != "auth status --json" {
		t.Errorf("calls = %v, want the sign-out followed by a status check", calls)
	}
	f.assertCleanEnv(dir)
}

// the status check that ends a sign-out is the last call, so assertCleanEnv above
// sees that one. the sign-out call has its own record.
func TestTheSignOutCallItselfRunsWithoutTheAuthVariables(t *testing.T) {
	f := newFake(t)
	f.say("logout", 0, "Signed out\n")
	f.say("status", 1, signedOutJSON)
	setAuthEnv(t)
	dir := t.TempDir()

	if err := Logout(context.Background(), dir); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	env := f.envOf("logout")
	for _, n := range authEnvVars {
		if _, ok := env[n]; ok {
			t.Errorf("%s reached claude's logout call", n)
		}
	}
	if env["CLAUDE_CONFIG_DIR"] != dir {
		t.Errorf("claude's logout call ran with CLAUDE_CONFIG_DIR=%q, want %q", env["CLAUDE_CONFIG_DIR"], dir)
	}
}

func TestLogoutFailsWhenTheFolderIsStillSignedIn(t *testing.T) {
	f := newFake(t)
	f.say("logout", 0, "")
	f.say("status", 0, signedInJSON)

	err := Logout(context.Background(), t.TempDir())

	if err == nil || !strings.Contains(err.Error(), "still signed in") {
		t.Fatalf("err = %v", err)
	}
	if errors.Is(err, ErrSignOutUnknown) {
		t.Error("a login that is known to be there is not an unknown state")
	}
}

func TestLogoutAcceptsAFolderWhoseClaudeAiLoginIsGone(t *testing.T) {
	f := newFake(t)
	f.say("logout", 0, "")
	// shared settings still provide an api key helper, which is not the account's login
	f.say("status", 0, `{"loggedIn":true,"authMethod":"api_key_helper","configDirectory":"@CONFIG_DIR@"}`)

	if err := Logout(context.Background(), t.TempDir()); err != nil {
		t.Errorf("Logout: %v", err)
	}
}

func TestLogoutThatClaudeReportedAsFailedIsNotUnknown(t *testing.T) {
	f := newFake(t)
	f.say("logout", 1, "")
	f.complain("logout", "no keychain")

	err := Logout(context.Background(), t.TempDir())

	if err == nil || errors.Is(err, ErrSignOutUnknown) {
		t.Errorf("err = %v, want a plain failure", err)
	}
}

func TestLogoutCutShortByTheContextIsUnknown(t *testing.T) {
	skipIfInterruptIgnored(t)
	f := newFake(t)
	f.hangOn("logout", "trap")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Logout(ctx, t.TempDir()) }()
	waitForFile(t, f, "ready")
	cancel()

	var err error
	select {
	case err = <-done:
	case <-time.After(patience()):
		t.Fatal("Logout did not return after the cancel")
	}

	var ce *CancelError
	if !errors.Is(err, ErrSignOutUnknown) || !errors.As(err, &ce) || !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want unknown, a CancelError and the context error", err)
	}
}

func TestLogoutWhoseCheckIsCutShortIsUnknown(t *testing.T) {
	skipIfInterruptIgnored(t)
	f := newFake(t)
	f.say("logout", 0, "")
	f.hangOn("status", "trap")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Logout(ctx, t.TempDir()) }()
	waitForFile(t, f, "ready")
	cancel()

	var err error
	select {
	case err = <-done:
	case <-time.After(patience()):
		t.Fatal("Logout did not return after the cancel")
	}

	if !errors.Is(err, ErrSignOutUnknown) || !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want unknown and the context error", err)
	}
}

func TestLogoutWhoseCheckCannotBeReadIsUnknown(t *testing.T) {
	f := newFake(t)
	f.say("logout", 0, "")
	f.say("status", 0, "not json")

	if err := Logout(context.Background(), t.TempDir()); !errors.Is(err, ErrSignOutUnknown) {
		t.Errorf("err = %v, want unknown", err)
	}
}
