package cli

import (
	"strings"
	"testing"
)

func TestRemovedCommandsExitTwoWithOneLine(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"add", "work"}, "clalt login <name>"},
		{[]string{"refresh", "work"}, "claude refreshes its own login"},
		{[]string{"refresh", "--all"}, "clalt login <name>"},
		{[]string{"keepalive", "status"}, "clalt doctor"},
		{[]string{"export", "backup.clalt"}, "no secrets"},
		{[]string{"import", "backup.clalt"}, "no secrets"},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			e := newEnv(t)

			code := e.run(c.args...)

			assertOneLineRefusal(t, e, code, c.want)
		})
	}
}

func TestRemovedFlagsExitTwoWithOneLine(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"usage", "--live"}, "live limits return with the statusline feed"},
		{[]string{"usage", "work", "--live"}, "--live is gone"},
		{[]string{"dashboard", "--live"}, "token vault"},
		{[]string{"doctor", "--fix"}, "doctor only reports now, 'clalt run' repairs links"},
		{[]string{"run", "work", "--refresh"}, "run --refresh is gone"},
		{[]string{"run", "--isolated", "work"}, "run --isolated is gone"},
		{[]string{"run", "work", "--no-supervise"}, "no restart supervisor"},
		{[]string{"run", "work", "--link"}, "run --link is gone"},
		{[]string{"login", "work", "--magic-link", "https://claude.ai/x"}, "login --magic-link is gone"},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			e := newEnv(t)

			code := e.run(c.args...)

			assertOneLineRefusal(t, e, code, c.want)
		})
	}
}

// the flags of the old run belong to claude after the double dash
func TestRemovedFlagsAfterTheDoubleDashAreNotOurs(t *testing.T) {
	e := newEnv(t)

	// the account does not exist, so run fails, but not with the gone message
	code := e.run("run", "work", "--", "--refresh", "--isolated")

	if code == 2 || strings.Contains(e.errOut.String(), "is gone") {
		t.Errorf("exit %d, stderr %q: a flag after -- must pass through", code, e.errOut.String())
	}
}

func assertOneLineRefusal(t *testing.T, e *testEnv, code int, want string) {
	t.Helper()
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	got := strings.TrimSpace(e.errOut.String())
	if strings.Contains(got, "\n") || !strings.Contains(got, want) {
		t.Errorf("stderr = %q, want one line containing %q", got, want)
	}
	if e.out.Len() != 0 {
		t.Errorf("stdout = %q, want nothing", e.out.String())
	}
}
