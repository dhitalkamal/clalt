package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
)

// a state.json that v0.1.0 or an older clalt wrote was never checked, and the
// upgrade guide tells people to keep it. what it holds is printed as it is read,
// so a label, an email, a plan or an organization with an escape sequence in it
// must not reach the screen. the label of an account defaults to its id, and ids
// from before names were validated can be anything.
const hostileState = `{"accounts":{"ok":{` +
	`"label":"lab\u001b[31mel","email":"a\u001b[2K@example.com","plan":"max\u0007","org":"Evil\u001b[31m Inc\u202e",` +
	`"addedAt":"2026-01-01T00:00:00Z","signedInAt":"2026-10-01T00:00:00Z","dailyBudget":5}}}`

// the label as it is typed, with the escape in it
const hostileLabel = "lab\x1b[31mel"

func hostileStateEnv(t *testing.T) *testEnv {
	t.Helper()
	e := newEnv(t)
	plantState(t, hostileState)
	e.claude.plantLogin(accountDir(t, "ok"), "a@example.com")
	return e
}

func TestListCleansWhatTheStateFileHoldsInTextAndInJSON(t *testing.T) {
	e := hostileStateEnv(t)

	if code := e.run("list"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if out := e.out.String(); hasControl(out) || strings.ContainsRune(out, '\u202e') || !strings.Contains(out, "max") {
		t.Errorf("stdout = %q", out)
	}

	rows := listJSON(t, e)
	for _, field := range []string{"label", "email", "plan", "org"} {
		if v, _ := rows[0][field].(string); hasControl(v) || strings.ContainsRune(v, '\u202e') || v == "" {
			t.Errorf("%s = %q", field, v)
		}
	}
}

func TestWhoamiCleansWhatTheStateFileHolds(t *testing.T) {
	e := hostileStateEnv(t)
	t.Setenv("CLAUDE_CONFIG_DIR", accountDir(t, "ok"))

	if code := e.run("whoami"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if out := e.out.String(); hasControl(out) || strings.ContainsRune(out, '\u202e') || !strings.Contains(out, "folder:") {
		t.Errorf("stdout = %q", out)
	}
}

func TestWhoamiInTheFolderOfARemovedAccountCleansItsLabel(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"accounts":{"ok":{"label":"lab\u001b[31mel","addedAt":"2026-01-01T00:00:00Z","hidden":true}}}`)
	t.Setenv("CLAUDE_CONFIG_DIR", accountDir(t, "ok"))

	if code := e.run("whoami"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if out := e.out.String(); strings.Contains(out, "\x1b") || !strings.Contains(out, "lab") {
		t.Errorf("stdout = %q", out)
	}

	// the encoder escapes a raw control character, so it is the decoded value that
	// shows whether it was cleaned
	e.reset()
	if code := e.run("whoami", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	var got map[string]string
	if err := json.Unmarshal(e.out.Bytes(), &got); err != nil {
		t.Fatalf("not json: %v\n%s", err, e.out.String())
	}
	if hasControl(got["label"]) || !strings.Contains(got["label"], "lab") {
		t.Errorf("label = %q", got["label"])
	}
}

func TestUsageAndBudgetCleanWhatTheStateFileHolds(t *testing.T) {
	e := hostileStateEnv(t)
	e.writeTranscript(replyLine)

	for _, args := range [][]string{{"usage"}, {"budget"}} {
		e.reset()
		if code := e.run(args...); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, e.errOut.String())
		}
		if out := e.out.String(); hasControl(out) || strings.ContainsRune(out, '\u202e') || !strings.Contains(out, "lab") {
			t.Errorf("%v: stdout = %q", args, out)
		}
	}
	e.reset()
	if code := e.run("budget", hostileLabel, "--daily", "7"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if out := e.out.String(); hasControl(out) {
		t.Errorf("the line that says which account was updated: %q", out)
	}
	e.reset()
	e.run("usage", "--json")
	var rows []struct{ Name, Plan, Email string }
	if err := json.Unmarshal(e.out.Bytes(), &rows); err != nil || len(rows) == 0 {
		t.Fatalf("not json: %v\n%s", err, e.out.String())
	}
	for _, r := range rows {
		if hasControl(r.Name) || hasControl(r.Plan) || hasControl(r.Email) {
			t.Errorf("a row of the usage JSON has an escape in it: %+v", r)
		}
	}
}

// every line of the report starts with the label of the account it is about
func TestDoctorCleansTheLabelThatALineStartsWith(t *testing.T) {
	e := hostileStateEnv(t)

	e.run("doctor")
	if out := e.out.String(); strings.Contains(out, "\x1b") || !strings.Contains(out, "lab") {
		t.Errorf("stdout = %q", out)
	}

	e.reset()
	e.run("doctor", "--json")
	var rep doctorReport
	if err := json.Unmarshal(e.out.Bytes(), &rep); err != nil {
		t.Fatalf("not json: %v\n%s", err, e.out.String())
	}
	named := false
	for _, c := range rep.Checks {
		if hasControl(c.Scope) {
			t.Errorf("the %s line is about %q", c.Check, c.Scope)
		}
		named = named || strings.Contains(c.Scope, "lab")
	}
	if !named {
		t.Errorf("no line is about the account: %+v", rep.Checks)
	}
}

func TestSessionsCleanTheLabelOfAnAccountThatIsStillThere(t *testing.T) {
	e := hostileStateEnv(t)
	if err := profile.WriteSession(config.SessionsDir(), profile.Session{
		Account: "ok", PID: os.Getpid(), Cwd: "/tmp/x", StartedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"sessions"}, {"sessions", "--json"}} {
		e.reset()
		if code := e.run(args...); code != 0 {
			t.Fatalf("exit %d: %s", code, e.errOut.String())
		}
		if out := e.out.String(); strings.Contains(out, "\x1b") || !strings.Contains(out, "lab") {
			t.Errorf("%v: stdout = %q", args, out)
		}
	}
}

// the messages that name an account by its label print it where they print
// anything else: the line that says what happened to the folder, the history
// warning, the question before a remove
func TestMessagesThatNameAnAccountCleanItsLabel(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"accounts":{"ok":{"label":"lab\u001b[31mel","addedAt":"2026-01-01T00:00:00Z"}}}`)
	e.seedShared()
	e.claude.plantLogin(accountDir(t, "ok"), "a@example.com")
	unmountHistory(t, e)
	e.claude.signInAs("a@example.com")

	e.run("login", hostileLabel)
	e.answer("n\n")
	e.run("remove", hostileLabel)
	e.run("logout", hostileLabel)

	if all := e.out.String() + e.errOut.String(); strings.Contains(all, "\x1b") {
		t.Errorf("an escape reached the output: %q", all)
	}
	for _, want := range []string{"does not reach", "remove lab", "is signed out"} {
		if all := e.out.String() + e.errOut.String(); !strings.Contains(all, want) {
			t.Errorf("the output never said %q:\n%s", want, all)
		}
	}
}
