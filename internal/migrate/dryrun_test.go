package migrate

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRunDryRunWritesNothing(t *testing.T) {
	profiles, dest := dirs(t)
	root := filepath.Join(profiles, "work", "projects")
	put(t, root, "-p/new.jsonl", "fresh bytes")
	put(t, root, "-p/same.jsonl", "same")
	put(t, root, "-p/clash.jsonl", "mine")
	age(t, put(t, dest, "-p/same.jsonl", "same"))
	age(t, put(t, dest, "-p/clash.jsonl", "theirs"))
	sourceBefore, destBefore := snapshot(t, profiles), snapshot(t, dest)

	dry := mustRun(t, profiles, dest, Options{DryRun: true})
	p := only(t, dry)
	if !dry.DryRun || p.Copied != 1 || p.Bytes != 11 || p.AlreadyThere != 1 ||
		!reflect.DeepEqual(p.Conflicts, []Conflict{{"-p/clash.jsonl", ReasonDiffers}}) {
		t.Errorf("got %+v, want 1 to copy (11 bytes), 1 already there, 1 conflict", p)
	}
	sameSnapshot(t, sourceBefore, snapshot(t, profiles))
	sameSnapshot(t, destBefore, snapshot(t, dest))
	noTempFiles(t, dest)

	// the real run does exactly what the dry run said it would
	after := mustRun(t, profiles, dest, Options{})
	dry.DryRun = false
	if !reflect.DeepEqual(dry, after) {
		t.Errorf("dry run and real run disagree:\n  dry  %+v\n  real %+v", dry, after)
	}
}

func TestRunDryRunDoesNotCreateTheDestination(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")

	mustRun(t, profiles, dest, Options{DryRun: true})
	if _, err := os.Lstat(filepath.Dir(dest)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a dry run created %s: %v", filepath.Dir(dest), err)
	}
}

// a dry run writes nothing, so a later profile can only be judged against what
// an earlier one would have placed if the plan remembers it.
func TestRunDryRunAcrossProfilesMatchesRealRun(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "alpha/projects/-proj/memory/MEMORY.md", "alpha notes")
	put(t, profiles, "beta/projects/-proj/memory/MEMORY.md", "beta notes are different")
	put(t, profiles, "alpha/projects/-proj/sess1.jsonl", "same bytes")
	put(t, profiles, "beta/projects/-proj/sess1.jsonl", "same bytes")
	put(t, profiles, "beta/projects/-proj/only-beta.jsonl", "beta only")
	put(t, profiles, "alpha/projects/-proj/short.jsonl", "the start")
	put(t, profiles, "beta/projects/-proj/short.jsonl", "the start and the rest")

	dry := mustRun(t, profiles, dest, Options{DryRun: true})
	after := mustRun(t, profiles, dest, Options{})
	dry.DryRun = false
	if !reflect.DeepEqual(dry, after) {
		t.Errorf("dry run and real run disagree:\n  dry  %+v\n  real %+v", dry, after)
	}
	if want := (Totals{Copied: 4, Bytes: 11 + 10 + 9 + 9, AlreadyThere: 1, Conflicts: 2}); after.Total != want {
		t.Errorf("real total = %+v, want %+v", after.Total, want)
	}
}

// a file that cannot be read is never placed, so it must not hide the same path
// in a later profile from the plan.
func TestRunDryRunPlanIgnoresASourceThatFails(t *testing.T) {
	profiles, dest := dirs(t)
	bad := put(t, profiles, "alpha/projects/-p/a.jsonl", "mine")
	put(t, profiles, "beta/projects/-p/a.jsonl", "mine")
	lockOut(t, bad)

	dry := mustRun(t, profiles, dest, Options{DryRun: true})
	after := mustRun(t, profiles, dest, Options{})
	dry.DryRun = false
	if !reflect.DeepEqual(dry, after) {
		t.Errorf("dry run and real run disagree:\n  dry  %+v\n  real %+v", dry, after)
	}
	if after.Total.Failed != 1 || after.Total.Copied != 1 {
		t.Errorf("real total = %+v, want alpha failed and beta copied", after.Total)
	}
}

// stat would say the file is fine, but the real run has to open it.
func TestRunDryRunReportsAnUnreadableSourceToo(t *testing.T) {
	profiles, dest := dirs(t)
	bad := put(t, profiles, "work/projects/-p/bad.jsonl", "private words")
	lockOut(t, bad)

	p := only(t, mustRun(t, profiles, dest, Options{DryRun: true}))
	if p.Copied != 0 || len(p.Failed) != 1 || p.Failed[0].Path != "-p/bad.jsonl" {
		t.Errorf("got %+v, want bad.jsonl failed", p)
	}
}

func TestRunSecondRunCopiesNothing(t *testing.T) {
	profiles, dest := dirs(t)
	root := filepath.Join(profiles, "work", "projects")
	put(t, root, "-p/a.jsonl", "one")
	put(t, root, "-p/sub/b.jsonl", "two")
	put(t, root, "-q/c.jsonl", "")

	first := mustRun(t, profiles, dest, Options{})
	if first.Total.Copied != 3 {
		t.Fatalf("first run copied %d, want 3", first.Total.Copied)
	}
	after := snapshot(t, dest)

	second := mustRun(t, profiles, dest, Options{})
	if want := (Totals{AlreadyThere: 3}); second.Total != want {
		t.Errorf("second run total = %+v, want %+v", second.Total, want)
	}
	sameSnapshot(t, after, snapshot(t, dest))
}
