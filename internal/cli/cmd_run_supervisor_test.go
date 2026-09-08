package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/credential"
	"github.com/dhitalkamal/clalt/internal/oauth"
)

// TestShouldSupervise proves the supervisor only engages when: not explicitly
// disabled, not a one-shot -p/--print invocation, and the account actually
// has a refresh token to renew with.
func TestShouldSupervise(t *testing.T) {
	withRefresh := credential.Credential{AccessToken: "a", RefreshToken: "r", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()}
	noRefresh := credential.Credential{AccessToken: "a", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()}

	cases := []struct {
		name       string
		opts       runOpts
		claudeArgs []string
		cred       credential.Credential
		want       bool
	}{
		{"normal interactive with refresh token", runOpts{}, nil, withRefresh, true},
		{"explicitly disabled", runOpts{noSupervise: true}, nil, withRefresh, false},
		{"one-shot print long flag", runOpts{}, []string{"--print"}, withRefresh, false},
		{"one-shot print short flag", runOpts{}, []string{"-p"}, withRefresh, false},
		{"no refresh token to renew with", runOpts{}, nil, noRefresh, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSupervise(tc.opts, tc.claudeArgs, tc.cred); got != tc.want {
				t.Errorf("shouldSupervise(%+v, %v, %+v) = %v, want %v", tc.opts, tc.claudeArgs, tc.cred, got, tc.want)
			}
		})
	}
}

// TestLaunchArgsAppendsOwnedSessionID proves the first launch appends
// --session-id when the supervisor owns a session id, so a later restart has
// something to --resume.
func TestLaunchArgsAppendsOwnedSessionID(t *testing.T) {
	got := launchArgs([]string{"--model", "opus"}, "sid-123")
	want := []string{"--model", "opus", "--session-id", "sid-123"}
	if !equalArgs(got, want) {
		t.Errorf("launchArgs = %v, want %v", got, want)
	}
}

// TestLaunchArgsUnchangedWithoutOwnedSessionID proves launchArgs leaves the
// original args untouched when the supervisor does not own a session id
// (e.g. the user already passed their own --resume/--continue).
func TestLaunchArgsUnchangedWithoutOwnedSessionID(t *testing.T) {
	original := []string{"--resume", "theirs"}
	got := launchArgs(original, "")
	if !equalArgs(got, original) {
		t.Errorf("launchArgs = %v, want unchanged %v", got, original)
	}
}

// TestRelaunchArgsAppendsResume proves a restart resumes the owned session id
// so conversation history carries over, without duplicating the original
// --session-id flag from the first launch.
func TestRelaunchArgsAppendsResume(t *testing.T) {
	got := relaunchArgs([]string{"--model", "opus"}, "sid-123")
	want := []string{"--model", "opus", "--resume", "sid-123"}
	if !equalArgs(got, want) {
		t.Errorf("relaunchArgs = %v, want %v", got, want)
	}
}

// TestRelaunchArgsUnchangedWithoutOwnedSessionID proves relaunchArgs does not
// fabricate a --resume when the supervisor never owned a session id -
// continuity may be lost, but the user's own flags are never fought.
func TestRelaunchArgsUnchangedWithoutOwnedSessionID(t *testing.T) {
	original := []string{"--continue"}
	got := relaunchArgs(original, "")
	if !equalArgs(got, original) {
		t.Errorf("relaunchArgs = %v, want unchanged %v", got, original)
	}
}

// TestNewOwnedSessionIDRespectsUserFlags proves the supervisor refuses to mint
// its own session id when the user is already managing continuity themselves.
func TestNewOwnedSessionIDRespectsUserFlags(t *testing.T) {
	if got := newOwnedSessionID([]string{"--resume", "theirs"}); got != "" {
		t.Errorf("newOwnedSessionID with user --resume = %q, want empty", got)
	}
}

// TestNewOwnedSessionIDGeneratesUUID proves the supervisor mints a distinct,
// well-formed UUID (v4, RFC 4122) when it owns continuity.
func TestNewOwnedSessionIDGeneratesUUID(t *testing.T) {
	a := newOwnedSessionID(nil)
	b := newOwnedSessionID(nil)
	if a == "" || b == "" {
		t.Fatal("newOwnedSessionID returned empty when it should own continuity")
	}
	if a == b {
		t.Errorf("newOwnedSessionID returned the same id twice: %q", a)
	}
	if len(a) != 36 {
		t.Errorf("newOwnedSessionID = %q, want 36-char UUID string", a)
	}
}

// TestRunSupervisorRestartsBeforeExpiry proves the transparent-restart
// supervisor end to end: it proactively refreshes an about-to-expire token,
// restarts the child claude process with the fresh token and --resume, and
// still returns cleanly once the (now long-lived) second child exits on its
// own - all with a stub claude binary, no real network or real claude needed.
func TestRunSupervisorRestartsBeforeExpiry(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the supervisor only engages on the darwin token-injection path")
	}
	app, _, dir := newTestApp(t)
	t.Setenv("XDG_CONFIG_HOME", dir)

	initial := credential.Credential{
		AccessToken: "tok-initial", RefreshToken: "r1",
		ExpiresAt: time.Now().Add(-time.Minute).UnixMilli(), // already expired -> resolveCred refreshes before launch
	}
	if err := app.Store.Save("work", initial, account.Meta{}); err != nil {
		t.Fatal(err)
	}
	// call 1 (resolveCred's pre-launch refresh) hands back a token that itself
	// expires almost immediately, so the supervisor's own timer fires quickly
	// during the first child's run; call 2 (the supervisor's mid-session
	// refresh) hands back a long-lived token so the loop settles after one
	// restart.
	var refreshCalls int
	app.refreshToken = func(ctx context.Context, refreshToken string) (oauth.TokenResult, error) {
		refreshCalls++
		if refreshCalls == 1 {
			return oauth.TokenResult{
				AccessToken: "tok-mid-session", RefreshToken: "r2",
				// 3s past the refresh buffer: gives the stub time to start and
				// log before the supervisor's timer fires and restarts it.
				ExpiresAt: time.Now().Add(refreshBuffer + 3*time.Second).UnixMilli(),
			}, nil
		}
		return oauth.TokenResult{
			AccessToken: "tok-refreshed", RefreshToken: "r3",
			ExpiresAt: time.Now().Add(time.Hour).UnixMilli(), // far enough out that it won't refire in this test
		}, nil
	}

	logPath := filepath.Join(dir, "invocations.log")
	stub := filepath.Join(dir, "claude-stub.sh")
	script := "#!/bin/bash\n" +
		"trap 'exit 0' TERM\n" +
		"{ echo \"ARGS:$*\"; echo \"TOKEN:$CLAUDE_CODE_OAUTH_TOKEN\"; echo '---'; } >> \"" + logPath + "\"\n" +
		"sleep 6 &\n" +
		"wait $!\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLALT_CLAUDE_BIN", stub)

	done := make(chan error, 1)
	go func() { done <- app.cmdRun([]string{"work"}) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("cmdRun: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for cmdRun to return")
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("stub claude never ran: %v", err)
	}
	invocations := strings.Split(strings.TrimRight(string(data), "\n"), "---\n")
	var nonEmpty []string
	for _, inv := range invocations {
		if strings.TrimSpace(inv) != "" {
			nonEmpty = append(nonEmpty, inv)
		}
	}
	if len(nonEmpty) != 2 {
		t.Fatalf("got %d claude invocations, want 2 (initial + supervised restart):\n%s", len(nonEmpty), data)
	}

	first, second := nonEmpty[0], nonEmpty[1]
	if !strings.Contains(first, "TOKEN:tok-mid-session") {
		t.Errorf("first invocation should carry the pre-launch-refreshed token:\n%s", first)
	}
	if !strings.Contains(first, "ARGS:--session-id ") {
		t.Errorf("first invocation should own a session id for later --resume:\n%s", first)
	}
	if !strings.Contains(second, "TOKEN:tok-refreshed") {
		t.Errorf("second invocation should carry the refreshed token:\n%s", second)
	}
	if !strings.Contains(second, "ARGS:--resume ") {
		t.Errorf("second invocation should resume the owned session id:\n%s", second)
	}

	// the same session id flows from the first launch's --session-id into the
	// second launch's --resume
	firstID := strings.TrimSpace(strings.SplitN(strings.TrimPrefix(strings.Split(first, "\n")[0], "ARGS:--session-id "), "\n", 2)[0])
	if !strings.Contains(second, firstID) {
		t.Errorf("restart did not resume the owned session id %q:\n%s", firstID, second)
	}
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestHasSessionFlag proves the supervisor detects when the user is already
// managing session continuity themselves, so it must not inject its own
// --session-id/--resume.
func TestHasSessionFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"none", []string{"--model", "opus"}, false},
		{"resume long", []string{"--resume", "abc"}, true},
		{"resume short", []string{"-r", "abc"}, true},
		{"continue long", []string{"--continue"}, true},
		{"continue short", []string{"-c"}, true},
		{"session id", []string{"--session-id", "abc"}, true},
		{"empty", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasSessionFlag(tc.args); got != tc.want {
				t.Errorf("hasSessionFlag(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

// TestParseRunArgsNoSupervise proves --no-supervise parses into runOpts and does
// not leak into the claude passthrough args.
func TestParseRunArgsNoSupervise(t *testing.T) {
	acct, opts, cargs, err := parseRunArgs([]string{"work", "--no-supervise"})
	if err != nil {
		t.Fatalf("parseRunArgs: %v", err)
	}
	if acct != "work" || !opts.noSupervise {
		t.Errorf("acct=%q opts=%+v, want noSupervise=true", acct, opts)
	}
	if len(cargs) != 0 {
		t.Errorf("claude args = %v, want none", cargs)
	}
}
