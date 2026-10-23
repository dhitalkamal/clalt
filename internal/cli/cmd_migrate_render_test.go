package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/migrate"
)

func prof(name string, status migrate.Status, edit func(*migrate.Profile)) migrate.Profile {
	p := migrate.Profile{
		Name:           name,
		Status:         status,
		Conflicts:      []migrate.Conflict{},
		SkippedEntries: []migrate.Skip{},
		Failed:         []migrate.Failure{},
	}
	if edit != nil {
		edit(&p)
	}
	return p
}

// report adds the totals up the way the migrate package does.
func report(dry bool, profiles ...migrate.Profile) migrate.Report {
	rep := migrate.Report{Source: "/src/profiles", Dest: "/shared/projects", DryRun: dry, Profiles: profiles}
	for _, p := range profiles {
		rep.Total.Copied += p.Copied
		rep.Total.Bytes += p.Bytes
		rep.Total.InPlace += p.InPlace
		rep.Total.AlreadyThere += p.AlreadyThere
		rep.Total.Conflicts += len(p.Conflicts)
		rep.Total.Skipped += p.Skipped
		rep.Total.Failed += len(p.Failed)
		if p.Status == migrate.StatusLink {
			rep.Total.Linked++
		}
	}
	return rep
}

func mustHave(t *testing.T, text string, lines ...string) {
	t.Helper()
	for _, want := range lines {
		if !strings.Contains(text, want) {
			t.Errorf("the report lacks %q:\n%s", want, text)
		}
	}
}

func TestRenderSaysWhyEachConflictIsOne(t *testing.T) {
	out := renderMigrate(report(false, prof("work", migrate.StatusFolder, func(p *migrate.Profile) {
		p.Conflicts = []migrate.Conflict{
			{Path: "-p/a.jsonl", Reason: migrate.ReasonDiffers},
			{Path: "-p/b.jsonl", Reason: migrate.ReasonInTheWay},
			{Path: "-p/c.jsonl", Reason: migrate.ReasonShorter},
		}
	})))
	mustHave(t, out,
		"conflict: -p/a.jsonl (the shared folder holds a different file)",
		"conflict: -p/b.jsonl (a folder, link or special file is in the way)",
		"conflict: -p/c.jsonl (the shared file is only the start of this one)",
	)
}

func TestRenderNamesEverySkippedEntry(t *testing.T) {
	out := renderMigrate(report(false, prof("work", migrate.StatusFolder, func(p *migrate.Profile) {
		p.Skipped = 7
		p.SkippedEntries = []migrate.Skip{
			{Path: "-p/l1", Kind: "link", Target: "/elsewhere/x"},
			{Path: "-p/l2", Kind: "link"},
			{Path: "-p/pipe", Kind: "pipe"},
			{Path: "-p/sock", Kind: "socket"},
			{Path: "-p/dev", Kind: "device"},
			{Path: "-p/odd", Kind: "irregular"},
			{Path: "-p/other", Kind: "special"},
		}
	})))
	mustHave(t, out,
		"skipped: -p/l1 (link to /elsewhere/x)",
		"skipped: -p/l2 (link)",
		"skipped: -p/pipe (pipe)",
		"skipped: -p/sock (socket)",
		"skipped: -p/dev (device)",
		"skipped: -p/odd (irregular file)",
		"skipped: -p/other (special file)",
	)
}

func TestRenderSaysWhetherAFileOrAFolderFailed(t *testing.T) {
	out := renderMigrate(report(false, prof("work", migrate.StatusFolder, func(p *migrate.Profile) {
		p.Failed = []migrate.Failure{
			{Path: "-p/a.jsonl", Kind: migrate.KindFile, Error: "open x: permission denied"},
			{Path: "-locked", Kind: migrate.KindFolder, Error: "open y: permission denied"},
		}
	})))
	mustHave(t, out,
		"failed file: -p/a.jsonl: open x: permission denied",
		"failed folder: -locked: open y: permission denied",
	)
}

func TestRenderShowsEveryKindOfLink(t *testing.T) {
	out := renderMigrate(report(false,
		prof("alpha", migrate.StatusLink, func(p *migrate.Profile) {
			p.LinkOf, p.Target = migrate.LinkOfProjects, "/elsewhere"
		}),
		prof("beta", migrate.StatusLink, func(p *migrate.Profile) {
			p.LinkOf, p.Target, p.Error = migrate.LinkOfProjects, "/gone", "lstat /gone: no such file or directory"
		}),
		prof("gamma", migrate.StatusLink, func(p *migrate.Profile) {
			p.LinkOf, p.Target = migrate.LinkOfProfile, "/away"
		}),
		prof("delta", migrate.StatusLink, func(p *migrate.Profile) {
			p.LinkOf, p.Target, p.Error = migrate.LinkOfProfile, "/gone", "lstat /gone: no such file or directory"
		}),
	))
	mustHave(t, out,
		"projects is a link to /elsewhere, not copied",
		"projects is a link to /gone that cannot be resolved (lstat /gone: no such file or directory), not copied",
		"profile folder is a link to /away, not scanned",
		"profile folder is a link to /gone that cannot be resolved (lstat /gone: no such file or directory), not scanned",
		"4 links not followed",
	)
}

func TestRenderCountsFilesWrittenInPlace(t *testing.T) {
	out := renderMigrate(report(false, prof("work", migrate.StatusFolder, func(p *migrate.Profile) {
		p.Copied, p.Bytes, p.InPlace = 3, 30, 2
	})))
	if got := strings.Count(out, "2 written in place"); got != 2 {
		t.Errorf("%d mentions of the in-place count, want one in the profile line and one in the total:\n%s", got, out)
	}
}

func TestRenderCleansLinkTargetsAndErrors(t *testing.T) {
	out := renderMigrate(report(false,
		prof("a", migrate.StatusLink, func(p *migrate.Profile) {
			p.LinkOf, p.Target, p.Error = migrate.LinkOfProfile, "t\x1b[2J", "why\nforged: yes"
		}),
		prof("b", migrate.StatusFolder, func(p *migrate.Profile) {
			p.Skipped = 1
			p.SkippedEntries = []migrate.Skip{{Path: "x\ny", Kind: "link", Target: "z\rq"}}
		}),
	))
	onlyOurNewlines(t, "the report", out)
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "forged:") || strings.HasPrefix(line, "y") || strings.HasPrefix(line, "q") {
			t.Errorf("a name or error forged the line %q:\n%s", line, out)
		}
	}
}

// errors.Join puts a newline between the errors it joins, and escaping that as
// \x0a would turn one failure into unreadable text. the lines are joined with
// "; " first, which also leaves a hostile message no line break to forge with.
func TestRenderJoinsAMultiLineErrorOnOneLine(t *testing.T) {
	out := renderMigrate(report(false, prof("work", migrate.StatusFolder, func(p *migrate.Profile) {
		p.Failed = []migrate.Failure{
			{Path: "-p/a.jsonl", Kind: migrate.KindFile, Error: "a\nb"},
			{Path: "-p/b.jsonl", Kind: migrate.KindFile, Error: "first\nforged: yes"},
		}
	})))
	mustHave(t, out, "failed file: -p/a.jsonl: a; b", "failed file: -p/b.jsonl: first; forged: yes")
	if strings.Contains(out, `\x0a`) {
		t.Errorf("a joined error should not show an escaped newline:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "forged:") {
			t.Errorf("an error forged the line %q:\n%s", line, out)
		}
	}
}

// the json report keeps the error exactly as it came, newline and all.
func TestJSONKeepsTheNewlineOfAJoinedError(t *testing.T) {
	app, out, _ := newTestApp(t)
	rep := report(false, prof("work", migrate.StatusFolder, func(p *migrate.Profile) {
		p.Failed = []migrate.Failure{{Path: "-p/a.jsonl", Kind: migrate.KindFile, Error: "a\nb"}}
	}))
	if err := app.emitJSON(rep); err != nil {
		t.Fatal(err)
	}
	var got migrate.Report
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if e := got.Profiles[0].Failed[0].Error; e != "a\nb" {
		t.Errorf("json error = %q, want the raw %q", e, "a\nb")
	}
}
