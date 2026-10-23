package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/account"
)

// addAccount makes an account without signing it in: rules are about where an
// account may be used, and need no login.
func addAccount(t *testing.T, e *testEnv, label string) {
	t.Helper()
	if _, err := e.app.createAccount(label); err != nil {
		t.Fatal(err)
	}
}

func (e *testEnv) rulesOf(label string) account.Rules {
	e.t.Helper()
	r, err := e.app.Store.RulesOf(e.account(label).Name)
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

// setRules gives an account rules without going through the command.
func (e *testEnv) setRules(label string, r account.Rules) {
	e.t.Helper()
	if err := e.app.Store.UpdateRules(e.account(label).Name, func(account.Rules) (account.Rules, error) { return r, nil }); err != nil {
		e.t.Fatal(err)
	}
}

func subfolder(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRulesShowsAnAccountThatHasNone(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")

	if code := e.run("rules", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	out := e.out.String()
	for _, want := range []string{"work", "folders:", "any (no rules)", "auto-switch:", "off"} {
		if !strings.Contains(out, want) {
			t.Errorf("the output has no %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "updated") {
		t.Errorf("a plain show said it updated something:\n%s", out)
	}
}

func TestRulesAddFolderStoresAnAbsolutePathAndShowsIt(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")
	dir := subfolder(t, "work-projects")

	if code := e.run("rules", "work", "--add-folder", dir); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := e.rulesOf("work").Folders; !reflect.DeepEqual(got, []string{dir}) {
		t.Errorf("folders = %v, want [%s]", got, dir)
	}
	out := e.out.String()
	if !strings.Contains(out, "rules updated for work") || !strings.Contains(out, dir) || strings.Contains(out, "any (no rules)") {
		t.Errorf("the output:\n%s", out)
	}
}

func TestRulesAddTakesARelativePathAgainstTheWorkingDirectory(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")
	dir := subfolder(t, "here")
	t.Chdir(dir)

	if code := e.run("rules", "work", "--add-folder", "."); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	got := e.rulesOf("work").Folders
	if len(got) != 1 || !filepath.IsAbs(got[0]) || filepath.Base(got[0]) != "here" {
		t.Errorf("folders = %v, want the absolute path of the working directory", got)
	}
}

func TestRulesAddOfTheSameFolderTwiceKeepsOne(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")
	dir := subfolder(t, "work-projects")

	for _, spelling := range []string{dir, dir + string(filepath.Separator), filepath.Join(dir, "..", filepath.Base(dir))} {
		if code := e.run("rules", "work", "--add-folder", spelling); code != 0 {
			t.Fatalf("exit %d: %s", code, e.errOut.String())
		}
	}

	if got := e.rulesOf("work").Folders; len(got) != 1 {
		t.Errorf("folders = %v, want one", got)
	}
}

func TestRulesAddSeveralFoldersAtOnceKeepsTheirOrder(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")
	a, b := subfolder(t, "a"), subfolder(t, "b")

	if code := e.run("rules", "work", "--add-folder", b, "--add-folder", a); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := e.rulesOf("work").Folders; !reflect.DeepEqual(got, []string{b, a}) {
		t.Errorf("folders = %v, want [%s %s]", got, b, a)
	}
}

func TestRulesRemoveFolderFindsItWhateverTheSpelling(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")
	keep, drop := subfolder(t, "keep"), subfolder(t, "drop")
	e.setRules("work", account.Rules{Folders: []string{keep, drop}})

	if code := e.run("rules", "work", "--remove-folder", drop+string(filepath.Separator)); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := e.rulesOf("work").Folders; !reflect.DeepEqual(got, []string{keep}) {
		t.Errorf("folders = %v, want [%s]", got, keep)
	}
}

func TestRulesRemoveFolderFindsALinkAndItsTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need developer mode on windows")
	}
	e := newEnv(t)
	addAccount(t, e, "work")
	real := subfolder(t, "real")
	link := filepath.Join(filepath.Dir(real), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	e.setRules("work", account.Rules{Folders: []string{real}})

	if code := e.run("rules", "work", "--remove-folder", link); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := e.rulesOf("work").Folders; len(got) != 0 {
		t.Errorf("folders = %v, want none", got)
	}
}

// a mistyped folder must not look like it worked
func TestRulesRemoveOfAFolderThatHasNoRuleIsAnErrorAndChangesNothing(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")
	have, other := subfolder(t, "have"), subfolder(t, "other")
	e.setRules("work", account.Rules{Folders: []string{have}, AutoSwitch: true})

	code := e.run("rules", "work", "--remove-folder", other, "--no-auto-switch")

	if code != 1 || !strings.Contains(e.errOut.String(), "no rule for "+other) {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if got := e.rulesOf("work"); !reflect.DeepEqual(got, account.Rules{Folders: []string{have}, AutoSwitch: true}) {
		t.Errorf("rules = %+v: a refused command changed them", got)
	}
}

func TestRulesRemovingTheLastFolderSaysTheAccountMayServeAnyFolder(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")
	dir := subfolder(t, "only")
	e.setRules("work", account.Rules{Folders: []string{dir}})

	if code := e.run("rules", "work", "--remove-folder", dir); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if msg := e.errOut.String(); !strings.Contains(msg, "may serve any folder") {
		t.Errorf("stderr does not warn that the rules are gone:\n%s", msg)
	}
	if !strings.Contains(e.out.String(), "any (no rules)") {
		t.Errorf("the output:\n%s", e.out.String())
	}
}

func TestRulesAutoSwitchOnPointsAtTheTermsAndOffClearsIt(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")

	if code := e.run("rules", "work", "--auto-switch"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if !e.rulesOf("work").AutoSwitch {
		t.Error("auto-switch was not turned on")
	}
	msg := e.errOut.String()
	for _, want := range []string{"terms", "docs/move.md"} {
		if !strings.Contains(msg, want) {
			t.Errorf("stderr has no %q:\n%s", want, msg)
		}
	}

	e.reset()
	if code := e.run("rules", "work", "--no-auto-switch"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if e.rulesOf("work").AutoSwitch {
		t.Error("auto-switch was not turned off")
	}
	if strings.Contains(e.errOut.String(), "terms") {
		t.Errorf("turning it off printed the terms:\n%s", e.errOut.String())
	}
}

func TestRulesMistakesAreUsageErrorsAndChangeNothing(t *testing.T) {
	dir := subfolder(t, "x")
	cases := [][]string{
		{"rules"},
		{"rules", "work", "--nope"},
		{"rules", "work", "--add-folder"},
		{"rules", "work", "--auto-switch", "--no-auto-switch"},
		// adding and removing one folder in one go says nothing about what is meant
		{"rules", "work", "--add-folder", dir, "--remove-folder", dir},
		{"rules", "work", "--add-folder", "~/projects"},
		{"rules", "work", "--add-folder", ""},
	}
	for _, args := range cases {
		e := newEnv(t)
		addAccount(t, e, "work")

		code := e.run(args...)

		if code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt rules <name>") {
			t.Errorf("%q: exit %d, stderr %q, want 2 and the usage line", args, code, e.errOut.String())
		}
		if got := e.rulesOf("work"); len(got.Folders) != 0 || got.AutoSwitch {
			t.Errorf("%q changed the rules: %+v", args, got)
		}
	}
}

func TestRulesAddOfAFolderThatIsNotThereWarnsAndKeepsIt(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")
	gone := filepath.Join(t.TempDir(), "not-yet")

	if code := e.run("rules", "work", "--add-folder", gone); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := e.rulesOf("work").Folders; !reflect.DeepEqual(got, []string{gone}) {
		t.Errorf("folders = %v", got)
	}
	if msg := e.errOut.String(); !strings.Contains(msg, "warning") || !strings.Contains(msg, "does not exist") {
		t.Errorf("no warning about the folder:\n%s", msg)
	}
	if out := e.out.String(); !strings.Contains(out, "(does not exist)") {
		t.Errorf("the listing does not mark the folder:\n%s", out)
	}
}
