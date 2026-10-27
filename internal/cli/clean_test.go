package cli

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"

	"github.com/dhitalkamal/clalt/internal/claudecli"
)

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"plain-name_1.jsonl":     "plain-name_1.jsonl",
		"-Users-x-app/a b.jsonl": "-Users-x-app/a b.jsonl",
		"Example Org":            "Example Org",
		"":                       "",
		"caf\u00e9":              "caf\u00e9",
		"\u540d\u524d":           "\u540d\u524d",
		"a\nb":                   `a\x0ab`,
		"a\rb":                   `a\x0db`,
		"a\tb":                   `a\x09b`,
		"a\x00b":                 `a\x00b`,
		"a\x07b\x08c":            `a\x07b\x08c`,
		"a\x7fb":                 `a\x7fb`,
		"a\x1b[31mb":             `a\x1b[31mb`,
		"a\u009bb":               `a\x9bb`,
		"a\u202eb":               `a\u202eb`,
		"a\u2066b\u2069c":        `a\u2066b\u2069c`,
		"a\u200bb":               `a\u200bb`,
		"a\u2028b\u2029c":        `a\u2028b\u2029c`,
		"a\U000e0001b":           `a\U000e0001b`,
		"a\xffb":                 `a\xffb`,
		// the other format characters, and the replacement character, which is
		// what a decoder shows for a byte it could not read and is kept
		"a\u061cb": `a\u061cb`,
		"a\u2060b": `a\u2060b`,
		"a\u200eb": `a\u200eb`,
		"a\u200fb": `a\u200fb`,
		"a\ufeffb": `a\ufeffb`,
		"a\u00adb": `a\xadb`,
		"a\ufffdb": "a\ufffdb",
		// the joiners are format characters too, but scripts and emoji
		// sequences need them and they cannot move a cursor
		"\U0001F468\u200d\U0001F469":                       "\U0001F468\u200d\U0001F469",
		"\u0645\u06cc\u200c\u062e\u0648\u0627\u0647\u0645": "\u0645\u06cc\u200c\u062e\u0648\u0627\u0647\u0645",
	} {
		if got := clean(in); got != want {
			t.Errorf("clean(%q) = %q, want %q", in, got, want)
		}
		// what is stored is cleaned again each time it is read back and compared
		if again := clean(clean(in)); again != clean(in) {
			t.Errorf("clean is not stable on %q: %q then %q", in, clean(in), again)
		}
	}
}

func TestCleanErrKeepsTheMessageAndEscapesIt(t *testing.T) {
	err := cleanErr(errors.New("bad\x1b[2J\nforged: yes"))
	if got, want := err.Error(), `bad\x1b[2J\x0aforged: yes`; got != want {
		t.Errorf("cleanErr = %q, want %q", got, want)
	}
}

// cleanStatus is where claude's answer is made safe on its way in. the warning
// is claude's text like the rest. the auth method and the folder are only
// compared, so they stay as claude said them.
func TestCleanStatusCleansTheTextClaudeReportsAndLeavesWhatIsCompared(t *testing.T) {
	in := claudecli.AuthStatus{
		Email: "a\x1b[31m@example.com", OrgName: "o\x07rg", SubscriptionType: "ma\nx", Warning: "w\x1b[2Jarn",
		AuthMethod: "claude\x1b.ai", ConfigDirectory: "/f\x07older",
	}
	want := claudecli.AuthStatus{
		Email: `a\x1b[31m@example.com`, OrgName: `o\x07rg`, SubscriptionType: `ma\x0ax`, Warning: `w\x1b[2Jarn`,
		AuthMethod: in.AuthMethod, ConfigDirectory: in.ConfigDirectory,
	}
	if got := cleanStatus(in); got != want {
		t.Errorf("cleanStatus = %+v, want %+v", got, want)
	}
}

// hasControl reports whether text could move a terminal around.
func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' {
			return true
		}
	}
	return false
}

// the organization name comes from a third party, and an email or plan could
// carry the same. none of it may reach the state file or the screen with an
// escape sequence in it.
func TestWhatClaudeReportsIsCleanedBeforeItIsStoredOrShown(t *testing.T) {
	e := newEnv(t)
	e.claude.set("ORG", `\u001b[31mEvil\u001b[0m Inc\u202e`)
	e.claude.set("PLAN", `max\u0007`)
	e.claude.signInAs(`a\u001b[2K@example.com`)

	if code := e.run("login", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	m := e.account("work").Meta
	for _, field := range []string{m.Email, m.Org, m.Plan} {
		if hasControl(field) || strings.ContainsRune(field, '\u202e') {
			t.Errorf("the state holds %q", field)
		}
	}
	if m.Email != `a\x1b[2K@example.com` || m.Org != `\x1b[31mEvil\x1b[0m Inc\u202e` || m.Plan != `max\x07` {
		t.Errorf("the state holds %q, %q, %q: a character that could move a terminal is shown as an escape", m.Email, m.Org, m.Plan)
	}
	raw, _ := os.ReadFile(filepath.Join(e.home, ".config", "clalt", "state.json"))
	if strings.Contains(string(raw), `\u001b`) || strings.ContainsRune(string(raw), '\u202e') {
		t.Errorf("state.json holds an escape:\n%s", raw)
	}
	if out := e.out.String(); hasControl(out) || strings.ContainsRune(out, '\u202e') {
		t.Errorf("login printed a control character: %q", out)
	}
	// the same login still matches itself: both sides were cleaned the same way
	e.reset()
	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("run exited %d: %s", code, e.errOut.String())
	}
	for _, c := range []struct {
		args   []string
		config string
	}{
		{[]string{"list"}, ""},
		{[]string{"whoami"}, e.folder("work")},
	} {
		e.reset()
		t.Setenv("CLAUDE_CONFIG_DIR", c.config)
		e.run(c.args...)
		if out := e.out.String(); hasControl(out) || strings.ContainsRune(out, '\u202e') {
			t.Errorf("%v printed a control character: %q", c.args, out)
		}
	}
}

func TestErrorLinesFromClaudeAreCleanedBeforeTheyAreShown(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("LOGOUT_FAILS", "1")
	e.claude.set("LOGOUT_ERR", `bad \0033[31mred\0033[0m \0033]0;owned\0007 thing`)
	e.claude.set("STATUS_BROKEN", "1")
	e.claude.set("STATUS_ERR", `status \0033[2Jfailed`)

	e.reset()
	if code := e.run("logout", "work"); code != 1 {
		t.Fatalf("logout exit %d", code)
	}
	if hasControl(e.errOut.String()) || !strings.Contains(e.errOut.String(), "bad") {
		t.Errorf("logout stderr: %q", e.errOut.String())
	}
	if row := listJSON(t, e)[0]; hasControl(row["reason"].(string)) || !strings.Contains(row["reason"].(string), "failed") {
		t.Errorf("list reason: %q", row["reason"])
	}
	_, rep := doctor(t, e)
	c, _ := rep.find("work", "signedIn")
	if hasControl(c.Detail) || !strings.Contains(c.Detail, "failed") {
		t.Errorf("doctor detail: %q", c.Detail)
	}
}

func TestNamesFromTheFileSystemAreCleanedInTheSyncReport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file names cannot hold an escape character here")
	}
	e := newEnv(t)
	home := e.seedShared()
	name := "x\x1b[31mred"
	// folders in the shared home are linked into every account
	if err := os.Mkdir(filepath.Join(home, name), 0o755); err != nil {
		t.Skipf("this file system refuses the name: %v", err)
	}
	e.signIn("work", "w@example.com")
	// a real folder where the link belongs is kept, and kept names are printed
	link := filepath.Join(e.folder("work"), name)
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(link, "keep"), 0o755); err != nil {
		t.Fatal(err)
	}

	e.reset()
	e.run("run", "work")

	if msg := e.errOut.String(); !strings.Contains(msg, "red") || !strings.Contains(msg, "in the way") {
		t.Fatalf("the kept name is not reported at all:\n%q", msg)
	}
	if hasControl(e.out.String()) || hasControl(e.errOut.String()) {
		t.Errorf("a file name put an escape on the screen: %q %q", e.out.String(), e.errOut.String())
	}
}
