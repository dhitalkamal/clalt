package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/account"
)

func TestTheWrapperSeesTheAccountsWithTheirRulesAndSignIns(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	addAccount(t, e, "fresh")
	dir := subfolder(t, "work")
	e.setRules("work", account.Rules{Folders: []string{dir}, AutoSwitch: true})
	if code := e.run("remove", "fresh", "--force"); code != 0 {
		t.Fatalf("remove exited %d", code)
	}

	cands, err := moveAccounts{e.app}.Candidates()

	if err != nil || len(cands) != 1 {
		t.Fatalf("Candidates = %+v, %v, want the one account that was not removed", cands, err)
	}
	c := cands[0]
	if c.ID != e.account("work").Name || c.Label != "work" || !c.SignedIn || !c.AutoSwitch || len(c.Folders) != 1 || c.Folders[0] != dir {
		t.Errorf("candidate = %+v", c)
	}
}

func TestPrepareGetsAnAccountReadyAndKeepsWhatItSaysUntilClaudeIsGone(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.signIn("personal", "p@example.com")
	// a link that is not there yet, so the sync has something to say
	if err := os.Remove(filepath.Join(e.folder("personal"), "settings.json")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	t.Setenv("ANTHROPIC_API_KEY", "sk-secret")

	prep, err := moveAccounts{e.app}.Prepare(e.account("personal").Name, "/x")

	if err != nil {
		t.Fatal(err)
	}
	if prep.Label != "personal" || prep.Folder != e.folder("personal") {
		t.Errorf("prepared = %+v", prep)
	}
	joined := strings.Join(prep.Notes, "\n")
	if !strings.Contains(joined, "ANTHROPIC_API_KEY") || strings.Contains(joined, "sk-secret") {
		t.Errorf("notes = %q, want the names of what is not passed on and no values", joined)
	}
	if e.errOut.Len() != 0 {
		t.Errorf("preparing wrote %q to stderr, where claude's screen is", e.errOut.String())
	}
	if _, err := os.Lstat(filepath.Join(e.folder("personal"), "projects")); err != nil {
		t.Errorf("the account was not linked: %v", err)
	}
}

func TestPrepareRefusesWhatARunWouldRefuse(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *testEnv)
		id    func(e *testEnv) string
		cwd   string
		want  string
	}{
		{"an account that does not exist", nil, func(*testEnv) string { return "nobody" }, "/x", "nobody"},
		{"a removed account", func(e *testEnv) {
			e.run("remove", "personal", "--force")
		}, func(e *testEnv) string { return e.account("personal").Name }, "/x", "removed"},
		{"a folder the rules leave out", func(e *testEnv) {
			e.setRules("personal", account.Rules{Folders: []string{subfolder(e.t, "elsewhere")}})
		}, func(e *testEnv) string { return e.account("personal").Name }, "/x", "folder rules"},
		{"a login that is gone", func(e *testEnv) {
			e.claude.signOut(e.folder("personal"))
		}, func(e *testEnv) string { return e.account("personal").Name }, "/x", "clalt login personal"},
	}
	for _, c := range cases {
		e := newEnv(t)
		e.seedShared()
		e.signIn("personal", "p@example.com")
		if c.setup != nil {
			c.setup(e)
		}

		_, err := moveAccounts{e.app}.Prepare(c.id(e), c.cwd)

		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want %q", c.name, err, c.want)
		}
	}
}
