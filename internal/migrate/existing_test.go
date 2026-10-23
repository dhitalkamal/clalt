package migrate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRunIdenticalFileIsAlreadyThere(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "same words")
	age(t, put(t, dest, "-p/a.jsonl", "same words"))
	before := snapshot(t, dest)

	rep := mustRun(t, profiles, dest, Options{})
	p := only(t, rep)
	if p.AlreadyThere != 1 || p.Copied != 0 || len(p.Conflicts) != 0 || len(p.Failed) != 0 {
		t.Errorf("got %+v, want 1 already there", p)
	}
	if rep.Total.AlreadyThere != 1 {
		t.Errorf("total already there = %d, want 1", rep.Total.AlreadyThere)
	}
	sameSnapshot(t, before, snapshot(t, dest)) // not even the mtime moves
	noTempFiles(t, dest)
}

func TestRunConflictingFileIsListedAndLeftAlone(t *testing.T) {
	for name, theirs := range map[string]string{
		"other length": "a much longer and different transcript",
		"same length":  "mine!", // one byte apart from the source below
	} {
		t.Run(name, func(t *testing.T) {
			profiles, dest := dirs(t)
			put(t, profiles, "work/projects/-p/a.jsonl", "mine?")
			put(t, profiles, "work/projects/-p/sub/b.jsonl", "fresh")
			age(t, put(t, dest, "-p/a.jsonl", theirs))
			sourceBefore := snapshot(t, profiles)
			keep := snapshot(t, filepath.Join(dest, "-p"))

			rep := mustRun(t, profiles, dest, Options{})
			p := only(t, rep)
			if want := []Conflict{{"-p/a.jsonl", ReasonDiffers}}; !reflect.DeepEqual(p.Conflicts, want) {
				t.Errorf("conflicts = %v, want %v", p.Conflicts, want)
			}
			if p.Copied != 1 || p.AlreadyThere != 0 || len(p.Failed) != 0 {
				t.Errorf("got %+v, want only the fresh file copied", p)
			}
			if rep.Total.Conflicts != 1 {
				t.Errorf("total conflicts = %d, want 1", rep.Total.Conflicts)
			}
			got, _ := os.ReadFile(filepath.Join(dest, "-p", "a.jsonl"))
			if string(got) != theirs {
				t.Errorf("the destination file was changed to %q", got)
			}
			after := snapshot(t, filepath.Join(dest, "-p"))
			if keep["a.jsonl"] != after["a.jsonl"] {
				t.Errorf("destination file changed:\n  was %s\n  now %s", keep["a.jsonl"], after["a.jsonl"])
			}
			sameSnapshot(t, sourceBefore, snapshot(t, profiles))
			noTempFiles(t, dest)
		})
	}
}

// a link at the final name is never followed, even when it leads to the same
// bytes, so what it points at cannot be touched or vouched for.
func TestRunLinkAtTheDestinationIsAConflict(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "same words")
	elsewhere := put(t, t.TempDir(), "same.txt", "same words")
	symlink(t, elsewhere, filepath.Join(dest, "-p", "a.jsonl"))
	before := snapshot(t, filepath.Dir(elsewhere))

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if want := []Conflict{{"-p/a.jsonl", ReasonInTheWay}}; !reflect.DeepEqual(p.Conflicts, want) || p.Copied != 0 {
		t.Errorf("got %+v, want a conflict", p)
	}
	sameSnapshot(t, before, snapshot(t, filepath.Dir(elsewhere)))
}

func TestRunFolderAtTheDestinationIsAConflict(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "words")
	put(t, dest, "-p/a.jsonl/inside.txt", "a folder sits where the file goes")

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if want := []Conflict{{"-p/a.jsonl", ReasonInTheWay}}; !reflect.DeepEqual(p.Conflicts, want) || p.Copied != 0 {
		t.Errorf("got %+v, want a conflict", p)
	}
}
