package migrate

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRunConflictSaysWhy(t *testing.T) {
	const start = "the first part of a transcript"
	for name, c := range map[string]struct {
		mine  string
		there func(t *testing.T, dest string)
		want  Reason
	}{
		"same size, other bytes": {"mine?", func(t *testing.T, dest string) {
			put(t, dest, "-p/a.jsonl", "mine!")
		}, ReasonDiffers},
		"longer there": {"mine", func(t *testing.T, dest string) {
			put(t, dest, "-p/a.jsonl", "mine and more")
		}, ReasonDiffers},
		"shorter but not the start": {"abcdef", func(t *testing.T, dest string) {
			put(t, dest, "-p/a.jsonl", "xyz")
		}, ReasonDiffers},
		"shorter start of the source": {start + " and the rest", func(t *testing.T, dest string) {
			put(t, dest, "-p/a.jsonl", start)
		}, ReasonShorter},
		"empty file there": {"abc", func(t *testing.T, dest string) {
			put(t, dest, "-p/a.jsonl", "")
		}, ReasonShorter},
		"folder in the way": {"mine", func(t *testing.T, dest string) {
			put(t, dest, "-p/a.jsonl/inside.txt", "a folder sits where the file goes")
		}, ReasonInTheWay},
		"link in the way": {"mine", func(t *testing.T, dest string) {
			elsewhere := put(t, t.TempDir(), "same.txt", "mine")
			symlink(t, elsewhere, filepath.Join(dest, "-p", "a.jsonl"))
		}, ReasonInTheWay},
	} {
		for _, dry := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s, dry=%v", name, dry), func(t *testing.T) {
				profiles, dest := dirs(t)
				put(t, profiles, "work/projects/-p/a.jsonl", c.mine)
				c.there(t, dest)

				p := only(t, mustRun(t, profiles, dest, Options{DryRun: dry}))
				want := []Conflict{{Path: "-p/a.jsonl", Reason: c.want}}
				if !reflect.DeepEqual(p.Conflicts, want) || p.Copied != 0 {
					t.Errorf("dry=%v: got %+v, want conflicts %+v", dry, p, want)
				}
			})
		}
	}
}
