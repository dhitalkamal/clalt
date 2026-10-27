package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// replyDay is later on the day of replyLine, so that its usage is today's
var replyDay = time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

// usage and budget read the transcripts of the shared home. inside a `clalt run`
// session CLAUDE_CONFIG_DIR is an account folder, whose projects is a link to
// them, so they have to find the shared one and not read an empty answer from
// the link.
func accountWithUsage(t *testing.T) (*testEnv, float64) {
	t.Helper()
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.seedShared()
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(replyLine)
	want := usageRows(t, e, "work")[0]["tokens"]
	if want == float64(0) {
		t.Fatal("setup: the baseline has no tokens")
	}
	return e, want.(float64)
}

func TestUsageReadsTheSharedHistoryFromInsideAnAccountFolder(t *testing.T) {
	e, want := accountWithUsage(t)
	t.Setenv("CLAUDE_CONFIG_DIR", e.folder("work"))

	if got := usageRows(t, e, "work")[0]["tokens"]; got != want {
		t.Errorf("inside an account folder: tokens = %v, want %v", got, want)
	}
}

// the link in the account folder is not what usage depends on: an account made
// while the volume was away has none, and one with a folder of its own has other
// transcripts, which are not the shared history
func TestUsageDoesNotDependOnWhatIsAtProjectsInTheAccountFolder(t *testing.T) {
	e, want := accountWithUsage(t)
	t.Setenv("CLAUDE_CONFIG_DIR", e.folder("work"))
	link := filepath.Join(e.folder("work"), "projects")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}

	if got := usageRows(t, e, "work")[0]["tokens"]; got != want {
		t.Errorf("with no projects in the account folder: tokens = %v, want %v", got, want)
	}

	mustMkdir(t, filepath.Join(link, "-other"))
	if err := os.WriteFile(filepath.Join(link, "-other", "o.jsonl"), []byte(strings.Replace(replyLine, "msg_1", "msg_9", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := usageRows(t, e, "work")[0]["tokens"]; got != want {
		t.Errorf("with a folder of its own: tokens = %v, want the shared history only, %v", got, want)
	}
}

func TestBudgetReadsTheSharedHistoryFromInsideAnAccountFolder(t *testing.T) {
	e, _ := accountWithUsage(t)
	t.Setenv("CLAUDE_CONFIG_DIR", e.folder("work"))
	// the fixture reply is from 2026-08-24, so look at it on that day
	e.at(replyDay)

	rows := budgetRows(t, e)

	if len(rows) != 1 || rows[0]["todayCost"] == float64(0) {
		t.Errorf("budget rows = %v: the usage of the day was not read", rows)
	}
}

func TestUsageFollowsASharedProjectsFolderThatIsItselfALink(t *testing.T) {
	e, want := accountWithUsage(t)
	shared := filepath.Join(e.home, ".claude", "projects")
	moved := shared + ".moved"
	if err := os.Rename(shared, moved); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, moved, shared)

	if got := usageRows(t, e, "work")[0]["tokens"]; got != want {
		t.Errorf("projects is a link to a folder: tokens = %v, want %v", got, want)
	}
}

// numbers that could not be read are not zero. usage and budget say why and exit
// 1, and print no table of zeros.
func TestUsageAndBudgetSayWhenTheSharedHistoryCannotBeRead(t *testing.T) {
	cases := []struct {
		name   string
		damage func(t *testing.T, e *testEnv)
		want   string
	}{
		{"it cannot be opened", func(t *testing.T, e *testEnv) {
			if !makeUnlookable(t, filepath.Join(e.home, ".claude", "projects")) {
				t.Skip("no permission bits to take away")
			}
		}, "permission denied"},
		{"it is on a volume that is not mounted", func(t *testing.T, e *testEnv) {
			unmountHistory(t, e)
		}, "leads nowhere"},
	}
	for _, c := range cases {
		for _, args := range [][]string{{"usage"}, {"usage", "--json", "work"}, {"budget"}, {"budget", "--json"}} {
			t.Run(c.name+", "+strings.Join(args, " "), func(t *testing.T) {
				e, _ := accountWithUsage(t)
				c.damage(t, e)
				e.reset()

				code := e.run(args...)

				if code != 1 || !strings.Contains(e.errOut.String(), c.want) {
					t.Errorf("exit %d, stderr %q, want 1 and %q", code, e.errOut.String(), c.want)
				}
				if e.out.Len() != 0 {
					t.Errorf("numbers were printed although they could not be read:\n%s", e.out.String())
				}
			})
		}
	}
}

// a machine that has never saved a transcript has no usage, and that is fine
func TestUsageWithoutAnyHistoryYetIsNotAnError(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	if err := os.RemoveAll(filepath.Join(e.home, ".claude", "projects")); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"usage", "--json"}, {"budget", "--json"}} {
		e.reset()
		if code := e.run(args...); code != 0 {
			t.Errorf("%v: exit %d: %s", args, code, e.errOut.String())
		}
	}
}

// with no home there is no shared history to read, and the working directory is
// never a place to look for it
func TestUsageAndBudgetWithoutAHomeSayWhatIsMissing(t *testing.T) {
	e := newEnv(t)
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join(".claude", "projects", "-p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".claude", "projects", "-p", "s.jsonl"), []byte(replyLine), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"usage"}, {"budget"}} {
		e.reset()
		code := e.run(args...)
		if code != 1 || !strings.Contains(e.errOut.String(), "HOME") {
			t.Errorf("%v: exit %d, stdout %q, stderr %q: want 1 and a message about HOME", args, code, e.out.String(), e.errOut.String())
		}
	}
}
