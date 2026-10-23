package rules

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// tree makes <root>/work/app, <root>/work-old and <root>/play, and returns root.
func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"work/app", "work-old", "play"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestNoRulesAllowAnyFolder(t *testing.T) {
	root := tree(t)
	for _, folders := range [][]string{nil, {}} {
		if !Allows(folders, filepath.Join(root, "play")) {
			t.Errorf("Allows(%v) = false, want true: no rules means any folder", folders)
		}
	}
	// even a working directory that could not be read
	if !Allows(nil, "") {
		t.Error("no rules and no cwd: want true")
	}
}

func TestAFolderRuleMatchesThatFolderAndEverythingUnderIt(t *testing.T) {
	root := tree(t)
	rule := []string{filepath.Join(root, "work")}
	cases := []struct {
		cwd  string
		want bool
	}{
		{filepath.Join(root, "work"), true},
		{filepath.Join(root, "work", "app"), true},
		{filepath.Join(root, "work") + string(filepath.Separator), true},
		{filepath.Join(root, "play"), false},
		// a longer name that starts with the same letters is another folder
		{filepath.Join(root, "work-old"), false},
		{root, false},
		{"", false},
	}
	for _, c := range cases {
		if got := Allows(rule, c.cwd); got != c.want {
			t.Errorf("Allows(%q) = %v, want %v", c.cwd, got, c.want)
		}
	}
}

func TestAnyOneRuleIsEnough(t *testing.T) {
	root := tree(t)
	rules := []string{filepath.Join(root, "work"), filepath.Join(root, "play")}
	if !Allows(rules, filepath.Join(root, "play")) {
		t.Error("the second rule was not tried")
	}
	if Allows(rules, filepath.Join(root, "work-old")) {
		t.Error("a folder that no rule covers was allowed")
	}
}

func TestSymlinksAreResolvedOnBothSides(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need developer mode on windows")
	}
	root := tree(t)
	link := filepath.Join(root, "shortcut")
	if err := os.Symlink(filepath.Join(root, "work"), link); err != nil {
		t.Fatal(err)
	}
	// the rule names the link, the session is in the real folder
	if !Allows([]string{link}, filepath.Join(root, "work", "app")) {
		t.Error("a rule that is a link did not match the folder it leads to")
	}
	// the rule names the real folder, the session came in through the link
	if !Allows([]string{filepath.Join(root, "work")}, filepath.Join(link, "app")) {
		t.Error("a cwd below a link did not match the rule for the real folder")
	}
	// a link out of the allowed folder does not make the target allowed
	out := filepath.Join(root, "work", "out")
	if err := os.Symlink(filepath.Join(root, "play"), out); err != nil {
		t.Fatal(err)
	}
	if Allows([]string{filepath.Join(root, "work-old")}, out) {
		t.Error("a link led into a folder that no rule covers")
	}
}

// the file system decides what counts as the same name: on a case-insensitive
// one (the macOS and Windows default) Work and work are one folder
func TestCaseIsIgnoredWhereTheFileSystemIgnoresIt(t *testing.T) {
	root := tree(t)
	swapped := filepath.Join(root, "WORK")
	if _, err := os.Stat(swapped); err != nil {
		t.Skip("this file system tells Work and work apart")
	}
	if !Allows([]string{swapped}, filepath.Join(root, "work", "app")) {
		t.Error("a rule spelled in another case did not match")
	}
	if !Allows([]string{filepath.Join(root, "work")}, filepath.Join(swapped, "app")) {
		t.Error("a cwd spelled in another case did not match")
	}
}

func TestARuleThatIsNotAbsoluteNeverMatches(t *testing.T) {
	root := tree(t)
	t.Chdir(filepath.Join(root, "work"))
	if Allows([]string{"app"}, filepath.Join(root, "work", "app")) {
		t.Error("a relative rule matched")
	}
	// and the account that has only such rules is not allowed anywhere
	if Allows([]string{""}, filepath.Join(root, "play")) {
		t.Error("an empty rule matched")
	}
}

func TestARuleForAFolderThatIsGoneMatchesNothingElse(t *testing.T) {
	root := tree(t)
	gone := filepath.Join(root, "deleted")
	if Allows([]string{gone}, filepath.Join(root, "play")) {
		t.Error("a missing folder allowed another one")
	}
}

func TestNormalizeMakesAnAbsoluteCleanPath(t *testing.T) {
	root := tree(t)
	t.Chdir(filepath.Join(root, "work"))
	want := filepath.Join(root, "work", "app")
	for _, in := range []string{"app", "./app", "app/", filepath.Join(want, "..", "app"), want + string(filepath.Separator)} {
		got, err := Normalize(in)
		if err != nil || got != want {
			t.Errorf("Normalize(%q) = %q, %v, want %q", in, got, err, want)
		}
	}
}

// keeps the folder as the user wrote it, only made absolute and clean: where it
// leads is worked out when it is matched, so a link that moves is followed
func TestNormalizeDoesNotResolveLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need developer mode on windows")
	}
	root := tree(t)
	link := filepath.Join(root, "shortcut")
	if err := os.Symlink(filepath.Join(root, "work"), link); err != nil {
		t.Fatal(err)
	}
	got, err := Normalize(link)
	if err != nil || got != link {
		t.Errorf("Normalize(%q) = %q, %v: the link should stay", link, got, err)
	}
}

func TestNormalizeRefusesWhatCannotBeAFolderRule(t *testing.T) {
	cases := map[string]string{
		"":                "empty",
		"   ":             "empty",
		"~":               "~",
		"~/Projects/work": "~",
		"a\x00b":          "NUL",
	}
	for in, want := range cases {
		_, err := Normalize(in)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Normalize(%q) error = %v, want one that mentions %q", in, err, want)
		}
	}
}

func TestSameSeesThroughSpellingLinksAndCase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need developer mode on windows")
	}
	root := tree(t)
	link := filepath.Join(root, "shortcut")
	if err := os.Symlink(filepath.Join(root, "work"), link); err != nil {
		t.Fatal(err)
	}
	if !Same(filepath.Join(root, "work"), link) {
		t.Error("a link and its folder are the same place")
	}
	if !Same(filepath.Join(root, "work", "app", ".."), filepath.Join(root, "work")) {
		t.Error("an unclean spelling is the same place")
	}
	if Same(filepath.Join(root, "work"), filepath.Join(root, "work-old")) {
		t.Error("two folders were taken for one")
	}
	if Same("", "") {
		t.Error("two empty paths are not a place")
	}
}

func TestProblemsListsRulesThatCanMatchNothing(t *testing.T) {
	root := tree(t)
	t.Chdir(root)
	gone := filepath.Join(root, "deleted")
	got := Problems([]string{filepath.Join(root, "work"), gone, "work", ""})
	want := []Problem{
		{Folder: gone, Why: WhyGone},
		// it exists next to the working directory, but a rule is never read
		// against the working directory
		{Folder: "work", Why: WhyNotAbsolute},
		{Folder: "", Why: WhyNotAbsolute},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Problems = %v, want %v", got, want)
	}
	if got := Problems(nil); len(got) != 0 {
		t.Errorf("Problems(nil) = %v", got)
	}
}

// a folder that cannot be looked at is not known to be gone
func TestProblemsDoesNotCallAnUnreadableFolderGone(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a folder that the user cannot look into")
	}
	root := tree(t)
	locked := filepath.Join(root, "locked")
	if err := os.MkdirAll(filepath.Join(locked, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if got := Problems([]string{filepath.Join(locked, "inside")}); len(got) != 0 {
		t.Errorf("Problems = %v, want none: the folder may be there", got)
	}
}

// the shell keeps the path as typed after a cd through a link, so a folder that
// looks inside a rule can really be outside it
func TestALinkOutOfARuleFolderDoesNotCarryItsTargetIn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need developer mode on windows")
	}
	root := tree(t)
	escape := filepath.Join(root, "work", "escape")
	if err := os.Symlink(filepath.Join(root, "play"), escape); err != nil {
		t.Fatal(err)
	}
	if Allows([]string{filepath.Join(root, "work")}, escape) {
		t.Error("a cwd that leads out of the rule folder was allowed")
	}
	// a link that stays inside is fine
	inside := filepath.Join(root, "work", "short")
	if err := os.Symlink(filepath.Join(root, "work", "app"), inside); err != nil {
		t.Fatal(err)
	}
	if !Allows([]string{filepath.Join(root, "work")}, inside) {
		t.Error("a link inside the rule folder was refused")
	}
}
