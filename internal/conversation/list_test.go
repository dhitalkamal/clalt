package conversation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func uid(n int) string { return fmt.Sprintf("aaaaaaaa-0000-0000-0000-%012d", n) }

func msg(ts, text string) string {
	return `{"type":"user","timestamp":"` + ts + `","cwd":"/w/app","message":{"content":"` + text + `"}}`
}

func ids(cs []Conversation) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestListIgnoresWhatIsNotAConversation(t *testing.T) {
	root := t.TempDir()
	writeIn(t, root, "-w-app", uid(1), msg("2026-10-20T10:00:00Z", "real"))
	writeIn(t, root, "-w-app", "agent-abc", msg("2026-10-20T10:00:00Z", "subagent file"))
	writeIn(t, root, "-w-app", uid(2), `{"type":"file-history-snapshot"}`)
	nested := filepath.Join(root, "-w-app", uid(1), "subagents")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeIn(t, filepath.Join(root, "-w-app", uid(1)), "subagents", uid(3), msg("2026-10-20T10:00:00Z", "nested"))
	if err := os.WriteFile(filepath.Join(root, "-w-app", uid(4)+".txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "stray.jsonl"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := List(root, Options{})

	if err != nil || !equal(ids(got), []string{uid(1)}) {
		t.Errorf("got %v, %v", ids(got), err)
	}
}

func TestListOfAMissingRootIsEmpty(t *testing.T) {
	got, err := List(filepath.Join(t.TempDir(), "nope"), Options{})

	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestListOfARootThatCannotBeReadIsAnError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports reading a file as a folder as not found, which is an empty root")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := List(file, Options{}); err == nil {
		t.Error("a root that is a file was taken for an empty one")
	}
}

func TestListIsNewestActivityFirst(t *testing.T) {
	root := t.TempDir()
	writeIn(t, root, "-w-a", uid(1), msg("2026-10-18T10:00:00Z", "old"))
	writeIn(t, root, "-w-b", uid(2), msg("2026-10-22T10:00:00Z", "newest"))
	writeIn(t, root, "-w-a", uid(3), msg("2026-10-20T10:00:00Z", "middle"))

	got, _ := List(root, Options{})

	if want := []string{uid(2), uid(3), uid(1)}; !equal(ids(got), want) {
		t.Errorf("got %v, want %v", ids(got), want)
	}
}

// the files are picked by mtime, so a limit parses only the newest ones
func TestListWithALimitTakesTheNewestFiles(t *testing.T) {
	root := t.TempDir()
	for i := 1; i <= 5; i++ {
		p := writeIn(t, root, "-w-a", uid(i), msg("2026-10-20T10:00:00Z", "m"))
		when := time.Date(2026, 10, i, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}

	got, _ := List(root, Options{Limit: 2})

	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	set := map[string]bool{got[0].ID: true, got[1].ID: true}
	if !set[uid(5)] || !set[uid(4)] {
		t.Errorf("got %v, want the two newest files", ids(got))
	}
	if all, _ := List(root, Options{Limit: 0}); len(all) != 5 {
		t.Errorf("limit 0 gave %d, want all 5", len(all))
	}
}

func TestListFilterNeedsEveryWordInTheIdTitleFolderOrModel(t *testing.T) {
	root := t.TempDir()
	writeIn(t, root, "-w-app", uid(1), msg("2026-10-20T10:00:00Z", "Fix the Login bug"),
		`{"type":"assistant","message":{"model":"claude-opus-5-5"}}`)
	writeIn(t, root, "-w-app", uid(2), msg("2026-10-21T10:00:00Z", "write docs"))
	cases := []struct {
		words []string
		want  []string
	}{
		{[]string{"login"}, []string{uid(1)}},
		{[]string{"LOGIN", "fix"}, []string{uid(1)}},
		{[]string{"login", "docs"}, nil},
		{[]string{"/w/app"}, []string{uid(2), uid(1)}},
		{[]string{"opus"}, []string{uid(1)}},
		{[]string{"0000-000000000002"}, []string{uid(2)}},
	}
	for _, c := range cases {
		got, _ := List(root, Options{Words: c.words})
		if !equal(ids(got), c.want) {
			t.Errorf("words %v: got %v, want %v", c.words, ids(got), c.want)
		}
	}
}

// a filter has to look past the newest files, or an older conversation could
// never be found
func TestListFilterWithALimitReachesOlderFiles(t *testing.T) {
	root := t.TempDir()
	for i := 1; i <= 4; i++ {
		text := "other"
		if i == 1 {
			text = "needle"
		}
		p := writeIn(t, root, "-w-a", uid(i), msg("2026-10-20T10:00:00Z", text))
		when := time.Date(2026, 10, i, 0, 0, 0, 0, time.UTC)
		os.Chtimes(p, when, when)
	}

	got, _ := List(root, Options{Words: []string{"needle"}, Limit: 1})

	if !equal(ids(got), []string{uid(1)}) {
		t.Errorf("got %v", ids(got))
	}
}

func TestFindByIdOrPrefix(t *testing.T) {
	root := t.TempDir()
	writeIn(t, root, "-w-a", uid(1), msg("2026-10-20T10:00:00Z", "one"))
	writeIn(t, root, "-w-a", "bbbbbbbb-0000-0000-0000-000000000002", msg("2026-10-20T10:00:00Z", "two"))

	for _, prefix := range []string{uid(1), "aaaaaaaa", "AAAAAAAA-0000"} {
		c, err := Find(root, prefix)
		if err != nil || c.ID != uid(1) || c.Title != "one" {
			t.Errorf("Find(%q) = %+v, %v", prefix, c, err)
		}
	}
}

func TestFindSaysWhenNothingMatches(t *testing.T) {
	root := t.TempDir()
	writeIn(t, root, "-w-a", uid(1), msg("2026-10-20T10:00:00Z", "one"))
	writeIn(t, root, "-w-a", uid(2), `{"type":"file-history-snapshot"}`)

	for _, prefix := range []string{"cccccccc", uid(2), ""} {
		if _, err := Find(root, prefix); !errors.Is(err, ErrNotFound) {
			t.Errorf("Find(%q) err = %v, want ErrNotFound", prefix, err)
		}
	}
}

func TestFindListsTheMatchesOfAnAmbiguousPrefix(t *testing.T) {
	root := t.TempDir()
	writeIn(t, root, "-w-a", uid(1), msg("2026-10-20T10:00:00Z", "one"))
	writeIn(t, root, "-w-b", uid(2), msg("2026-10-21T10:00:00Z", "two"))

	_, err := Find(root, "aaaaaaaa")

	var amb *AmbiguousError
	if !errors.As(err, &amb) || len(amb.Matches) != 2 {
		t.Fatalf("err = %v", err)
	}
}

// claude calls a copy of one session in two projects not found, so it is
// ambiguous here too
func TestFindTreatsTheSameIdInTwoProjectsAsAmbiguous(t *testing.T) {
	root := t.TempDir()
	writeIn(t, root, "-w-a", uid(1), msg("2026-10-20T10:00:00Z", "one"))
	writeIn(t, root, "-w-b", uid(1), msg("2026-10-20T10:00:00Z", "copy"))

	_, err := Find(root, uid(1))

	var amb *AmbiguousError
	if !errors.As(err, &amb) {
		t.Fatalf("err = %v", err)
	}
}
