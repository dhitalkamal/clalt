package cli

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/migrate"
)

func folder(name string, edit func(*migrate.Profile)) migrate.Profile {
	return prof(name, migrate.StatusFolder, func(p *migrate.Profile) {
		p.Copied, p.Bytes = 1, 5
		if edit != nil {
			edit(p)
		}
	})
}

func withConflict(p *migrate.Profile) {
	p.Conflicts = []migrate.Conflict{{Path: "-p/a.jsonl", Reason: migrate.ReasonDiffers}}
}

func withFailure(p *migrate.Profile) {
	p.Failed = []migrate.Failure{{Path: "-p/b.jsonl", Kind: migrate.KindFile, Error: "boom"}}
}

func withLink(name string) migrate.Profile {
	return prof(name, migrate.StatusLink, func(p *migrate.Profile) {
		p.LinkOf, p.Target = migrate.LinkOfProjects, "/elsewhere"
	})
}

// each line after the total says one true thing, and appears only when it is
// true.
func TestRenderFootersFollowWhatHappened(t *testing.T) {
	const (
		kept     = "left as they were"
		conflict = "shared folder already holds a different file"
		failed   = "failed entries are listed above with their reasons"
		linked   = "links are never followed"
		inPlace  = "written in place because the destination has no hard links"
		partial  = "partial file under its real name"
	)
	for name, c := range map[string]struct {
		rep  migrate.Report
		have []string
		not  []string
	}{
		"clean": {report(false, folder("a", nil)),
			[]string{kept}, []string{conflict, failed, linked, inPlace}},
		"a conflict": {report(false, folder("a", withConflict)),
			[]string{kept, conflict, "only in the profile folder"}, []string{failed, linked, inPlace}},
		// a file can fail after it was placed, so a failure says nothing about
		// whether the file reached the shared folder.
		"a failure": {report(false, folder("a", withFailure)),
			[]string{kept, failed}, []string{conflict, linked, inPlace, "were not copied", "only in the profile folder"}},
		"a link": {report(false, withLink("a")),
			[]string{kept, linked}, []string{conflict, failed, inPlace}},
		"one file in place": {report(false, folder("a", func(p *migrate.Profile) { p.InPlace = 1 })),
			[]string{kept, "1 file was " + inPlace, partial}, []string{conflict, failed, linked}},
		"files in place": {report(false, folder("a", func(p *migrate.Profile) { p.InPlace = 3 })),
			[]string{"3 files were " + inPlace, partial}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			out := renderMigrate(c.rep)
			mustHave(t, out, c.have...)
			for _, no := range c.not {
				if strings.Contains(out, no) {
					t.Errorf("the report should not say %q:\n%s", no, out)
				}
			}
		})
	}
}

func TestRenderDryRunSaysWhatItDoesNotCheck(t *testing.T) {
	clean := renderMigrate(report(true, folder("a", nil)))
	mustHave(t, clean,
		"nothing was written",
		"does not check free space or write permission",
		"run 'clalt migrate' to copy for real",
	)

	failing := renderMigrate(report(true, folder("a", withFailure)))
	mustHave(t, failing,
		"nothing was written",
		"does not check free space or write permission",
		"the dry run found failures, see the failed list above",
	)
	if strings.Contains(failing, "for real") {
		t.Errorf("a dry run that found failures must not invite a real run:\n%s", failing)
	}
}

var oldWord = regexp.MustCompile(`(?i)\bold\b`)

// whichever branch merges first, per-account history is not old, and nothing
// here tells the user to delete anything.
func TestRenderUsesNoOldWordAndNeverTellsToRemove(t *testing.T) {
	rich := report(false,
		folder("a", func(p *migrate.Profile) { withConflict(p); withFailure(p); p.InPlace = 1 }),
		withLink("b"),
		prof("c", migrate.StatusNothing, nil),
		prof("d", migrate.StatusShared, nil),
	)
	for _, rep := range []migrate.Report{rich, report(true, folder("a", nil)), {Source: "/s"}} {
		out := renderMigrate(rep)
		if oldWord.MatchString(out) || strings.Contains(strings.ToLower(out), "remove") {
			t.Errorf("the report says old or remove:\n%s", out)
		}
	}
	app, help, _ := newTestApp(t)
	app.usage()
	lines := strings.Split(help.String(), "\n")
	for i, line := range lines {
		if strings.Contains(line, "clalt migrate") {
			text := line + "\n" + lines[i+1]
			if oldWord.MatchString(text) {
				t.Errorf("the help for migrate says old:\n%s", text)
			}
		}
	}
}

// the error after a dry run says it was a dry run, and points at the list.
func TestMigrateFailureErrorSaysIfItWasADryRun(t *testing.T) {
	profiles, _ := migrateHome(t)
	bad := filepath.Join(profiles, "work", "projects", "-p", "a.jsonl")
	migrateWrite(t, bad, "private words")
	if !lockOut(t, bad) {
		t.Skip("permission bits do not stop this user")
	}

	_, out, errOut := runMigrate(t, "--dry-run")
	if !strings.Contains(errOut, "dry run found 1 failure") || !strings.Contains(errOut, "failed list") || strings.Contains(errOut, "while copying") {
		t.Errorf("dry run stderr = %q", errOut)
	}
	if strings.Contains(out, "for real") {
		t.Errorf("a dry run with failures must not invite a real run:\n%s", out)
	}

	_, _, errOut = runMigrate(t)
	if !strings.Contains(errOut, "1 failure while copying") || !strings.Contains(errOut, "failed list") {
		t.Errorf("real run stderr = %q", errOut)
	}
}
