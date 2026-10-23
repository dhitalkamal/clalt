package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRunStartsClaudeInTheAccountFolder(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	t.Setenv("CLALT_TEST_KEEP", "kept")

	code := e.run("run", "work")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	argv, env, _ := e.claude.session()
	if len(argv) != 0 {
		t.Errorf("claude got arguments %v, want none", argv)
	}
	if env["CLAUDE_CONFIG_DIR"] != e.folder("work") {
		t.Errorf("CLAUDE_CONFIG_DIR = %q, want %q", env["CLAUDE_CONFIG_DIR"], e.folder("work"))
	}
	if env["CLALT_TEST_KEEP"] != "kept" {
		t.Error("an unrelated variable was dropped")
	}
	// there is no token to hand over any more
	for _, name := range []string{"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_SECURESTORAGE_CONFIG_DIR"} {
		if _, ok := env[name]; ok {
			t.Errorf("%s reached claude", name)
		}
	}
}

func TestRunPassesEverythingAfterTheNameToClaude(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"after the double dash", []string{"run", "work", "--", "--resume", "abc"}, []string{"--resume", "abc"}},
		{"before it, as today", []string{"run", "work", "--model", "opus"}, []string{"--model", "opus"}},
		{"a flag that used to be clalt's", []string{"run", "work", "--", "--refresh"}, []string{"--refresh"}},
		{"json after the double dash is claude's", []string{"run", "work", "--", "mcp", "list", "--json"}, []string{"mcp", "list", "--json"}},
		{"the double dash itself is not passed", []string{"run", "work", "--"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "a@example.com")

			if code := e.run(c.args...); code != 0 {
				t.Fatalf("exit %d: %s", code, e.errOut.String())
			}

			if argv, _, _ := e.claude.session(); !reflect.DeepEqual(argv, c.want) {
				t.Errorf("claude got %v, want %v", argv, c.want)
			}
		})
	}
}

func TestRunNamesTheStrippedVariablesAndStripsThem(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-secret")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")

	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	out := e.out.String() + e.errOut.String()
	if !strings.Contains(out, "ANTHROPIC_API_KEY, CLAUDE_CODE_USE_BEDROCK") || strings.Contains(out, "sk-test-secret") {
		t.Errorf("names only, please:\n%s", out)
	}
	if _, env, _ := e.claude.session(); env["ANTHROPIC_API_KEY"] != "" || env["CLAUDE_CODE_USE_BEDROCK"] != "" {
		t.Error("a stripped variable reached claude")
	}
}

func TestRunExitsWithClaudesOwnCodeAndSaysNothingMore(t *testing.T) {
	cases := map[string]int{"7": 7, "1": 1, "term": 143}
	for exit, want := range cases {
		t.Run("exit "+exit, func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "a@example.com")
			e.claude.set("EXIT", exit)

			code := e.run("run", "work")

			if code != want {
				t.Errorf("exit code = %d, want %d", code, want)
			}
			if got := e.errOut.String(); got != "" {
				t.Errorf("stderr = %q, want nothing: claude's exit is not clalt's error", got)
			}
		})
	}
}

func TestRunGivesClaudeTheTerminal(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.set("SESSION_PRINTS", "1")
	e.app.In = strings.NewReader("the prompt")

	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if !strings.Contains(e.out.String(), "hello from claude") {
		t.Errorf("claude's output did not reach stdout:\n%s", e.out.String())
	}
	if got := e.claude.sessionStdin(); got != "the prompt" {
		t.Errorf("claude read %q from stdin", got)
	}
}

func TestRunRecordsTheSessionUnderTheIdWhileItLasts(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	id := e.account("work").Name
	if err := e.app.Store.Relabel(id, "office"); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()

	if code := e.run("run", "office"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	rec := e.claude.sessionRecords()
	if !strings.Contains(rec, `"account":"`+id+`"`) || !strings.Contains(rec, cwd) {
		t.Errorf("the session record while claude ran: %q", rec)
	}
	left, _ := filepath.Glob(filepath.Join(e.home, ".config", "clalt", "sessions", "*.json"))
	if len(left) != 0 {
		t.Errorf("records left behind: %v", left)
	}
	// the label is the handle and the id the folder: renaming never moved it
	if _, env, _ := e.claude.session(); env["CLAUDE_CONFIG_DIR"] != filepath.Join(e.home, ".config", "clalt", "accounts", id) {
		t.Errorf("CLAUDE_CONFIG_DIR = %q", env["CLAUDE_CONFIG_DIR"])
	}
}

func TestRunRecordsLastUse(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	before := time.Now().Add(-time.Second)

	e.run("run", "work")

	if got := e.account("work").Meta.LastUsedAt; got.Before(before) {
		t.Errorf("lastUsedAt = %v, want a time of now", got)
	}
}

func TestRunFindsTheAccountWhateverTheCase(t *testing.T) {
	e := newEnv(t)
	e.signIn("Work", "a@example.com")

	if code := e.run("run", "WORK"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
}

func TestRunSaysWhatItSyncedOnlyWhenSomethingChanged(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com") // the login links everything

	e.run("run", "work")
	if msg := e.out.String() + e.errOut.String(); strings.Contains(msg, "synced") {
		t.Errorf("nothing changed, yet:\n%s", msg)
	}

	// a new folder appears in the shared home: the next run links it
	if err := os.Mkdir(filepath.Join(e.home, ".claude", "commands"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.reset()
	e.run("run", "work")
	if !strings.Contains(e.errOut.String(), "synced with the shared claude home: 1 item linked") {
		t.Errorf("the new link is not reported:\n%s", e.errOut.String())
	}
}

func TestRunRefusesWhenProjectsIsARealFolder(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	projects := filepath.Join(e.folder("work"), "projects")
	if err := os.Remove(projects); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projects, "-p"), 0o755); err != nil {
		t.Fatal(err)
	}

	code := e.run("run", "work")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "history would not be shared") || !strings.Contains(msg, projects) {
		t.Errorf("exit %d: %s", code, msg)
	}
	if e.claude.sessionStarted() {
		t.Error("claude started with a history that is not shared")
	}
}

func TestRunWarnsAboutAnItemThatCannotBeLinkedAndRefusesIfItIsProjects(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	folder := e.folder("work")
	for _, name := range []string{"projects", "settings.json"} {
		if err := os.Remove(filepath.Join(folder, name)); err != nil {
			t.Fatal(err)
		}
	}
	if !makeReadOnly(t, folder) {
		t.Skip("cannot make a folder read-only here")
	}

	code := e.run("run", "work")

	msg := e.errOut.String()
	for _, want := range []string{"could not link projects", "could not link settings.json", "history would not be shared", "projects could not be linked"} {
		if !strings.Contains(msg, want) {
			t.Errorf("stderr is missing %q:\n%s", want, msg)
		}
	}
	if code != 1 || e.claude.sessionStarted() {
		t.Errorf("exit %d, started %v", code, e.claude.sessionStarted())
	}
}

func TestRunInsideAnotherSessionStillSharesTheRealHome(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	e.signIn("home", "b@example.com")
	// a shell inside a `clalt run work` session
	t.Setenv("CLAUDE_CONFIG_DIR", e.folder("work"))

	if code := e.run("run", "home"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	link, err := os.Readlink(filepath.Join(e.folder("home"), "projects"))
	if err != nil || link != filepath.Join(e.home, ".claude", "projects") {
		t.Errorf("projects links to %q (%v)", link, err)
	}
	if _, env, _ := e.claude.session(); env["CLAUDE_CONFIG_DIR"] != e.folder("home") {
		t.Errorf("CLAUDE_CONFIG_DIR = %q", env["CLAUDE_CONFIG_DIR"])
	}
}
