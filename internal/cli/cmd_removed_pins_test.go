package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// what each retired call says, word for word. the hint is the part that tells
// the user what to do instead, so it is pinned and not only the "is gone".
var retiredCalls = []struct {
	args []string
	line string
}{
	{[]string{"add", "work"}, "clalt add is gone: use 'clalt login <name>'"},
	{[]string{"refresh", "work"}, "clalt refresh is gone: claude refreshes its own login, use 'clalt login <name>' if an account is signed out"},
	{[]string{"refresh", "--all"}, "clalt refresh is gone: claude refreshes its own login, use 'clalt login <name>' if an account is signed out"},
	{[]string{"keepalive", "status"}, "clalt keepalive is gone and no longer needed: 'clalt doctor' shows an old job that is still installed"},
	{[]string{"export", "backup.clalt"}, "clalt export is gone: accounts hold no secrets now, so there is nothing to back up"},
	{[]string{"import", "backup.clalt"}, "clalt import is gone: accounts hold no secrets now, so there is nothing to restore"},
	{[]string{"run", "work", "--refresh"}, "run --refresh is gone: claude refreshes its own login"},
	{[]string{"run", "--isolated", "work"}, "run --isolated is gone: every account shares your claude settings now"},
	{[]string{"run", "work", "--no-supervise"}, "run --no-supervise is gone: there is no restart supervisor any more"},
	{[]string{"run", "work", "--link"}, "run --link is gone: sharing is always on"},
	{[]string{"usage", "--live"}, "--live is gone: it needed the token vault. live limits return with the statusline feed"},
	{[]string{"dashboard", "--live"}, "--live is gone: it needed the token vault. live limits return with the statusline feed"},
	{[]string{"doctor", "--fix"}, "doctor --fix is gone: doctor only reports now, 'clalt run' repairs links"},
	{[]string{"login", "work", "--magic-link", "https://claude.ai/x"}, "login --magic-link is gone: claude signs in through its own browser flow now"},
}

// the flag --json can stand anywhere, before the command or after the retired
// flag, and a refusal that is meant for a script is the same one
func withJSON(args []string) [][]string {
	return [][]string{args, append([]string{"--json"}, args...), append(append([]string{}, args...), "--json")}
}

func TestEveryRetiredCallSaysWhatToUseInsteadAndExitsTwo(t *testing.T) {
	for _, c := range retiredCalls {
		for _, args := range withJSON(c.args) {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				e := newEnv(t)

				code := e.run(args...)

				if want := "error: " + c.line + "\n"; code != 2 || e.errOut.String() != want || e.out.Len() != 0 {
					t.Errorf("exit %d, stderr %q, stdout %q, want 2, %q and nothing", code, e.errOut.String(), e.out.String(), want)
				}
			})
		}
	}
}

// a refused call has no side effects. there is an account, it is signed in, and
// after the call it is as it was: its record, claude not asked anything, no session
// started, no file made in the working directory.
func TestARetiredCallChangesNothing(t *testing.T) {
	for _, c := range retiredCalls {
		for _, args := range withJSON(c.args) {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				e := newEnv(t)
				e.signIn("work", "w@example.com")
				state, err := os.ReadFile(config.StatePath())
				if err != nil {
					t.Fatal(err)
				}
				asked := func() int {
					return len(e.claude.calls("status")) + len(e.claude.calls("login")) + len(e.claude.calls("logout"))
				}
				before, hadSessions := asked(), exists(config.SessionsDir())
				t.Chdir(t.TempDir())
				here := snapshot(t, ".")

				e.reset()
				e.run(args...)

				if after, _ := os.ReadFile(config.StatePath()); string(after) != string(state) {
					t.Errorf("state.json changed:\n%s\nthen\n%s", state, after)
				}
				if got := asked(); got != before {
					t.Errorf("claude was asked %d more times", got-before)
				}
				if e.claude.sessionStarted() {
					t.Error("a session started")
				}
				if exists(config.SessionsDir()) != hadSessions {
					t.Error("the sessions folder was made or removed")
				}
				if now := snapshot(t, "."); strings.Join(now, "\n") != strings.Join(here, "\n") {
					t.Errorf("the working directory changed:\n%v\nthen\n%v", here, now)
				}
			})
		}
	}
}

// a flag that is gone for one command still belongs to claude after the name of
// a run
func TestAFlagThatIsGoneForAnotherCommandIsClaudes(t *testing.T) {
	for _, flag := range []string{"--live", "--fix", "--magic-link"} {
		e := newEnv(t)
		e.signIn("work", "w@example.com")

		if code := e.run("run", "work", flag); code != 0 {
			t.Fatalf("run work %s: exit %d: %s", flag, code, e.errOut.String())
		}

		if argv, _, _ := e.claude.session(); len(argv) != 1 || argv[0] != flag {
			t.Errorf("claude got %v, want [%s]", argv, flag)
		}
	}
}
