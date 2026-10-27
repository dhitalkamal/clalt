package activity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyMain copies the main fixture into a temp file so a test can append to it.
func copyMain(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/main.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "live.jsonl")
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func appendLines(t *testing.T, p string, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		t.Fatal(err)
	}
}

func reply(ts, text string) string {
	return `{"type":"assistant","timestamp":"` + ts + `","message":{"content":[{"type":"text","text":"` + text + `"}]}}`
}

func TestNewestIsGivenAndAfterReturnsOnlyNewerEntries(t *testing.T) {
	p := copyMain(t)
	first := readAll(t, Options{Source: Source{Path: p}})
	if first.Newest == "" {
		t.Fatal("a reply of the newest page needs a newest cursor")
	}
	appendLines(t, p, reply("2026-10-27T14:01:00Z", "one"), reply("2026-10-27T14:01:01Z", "two"))
	r := readAll(t, Options{Source: Source{Path: p}, After: first.Newest})
	if len(r.Entries) != 2 || r.Entries[0].Text != "two" || r.Entries[1].Text != "one" || r.Next != "" {
		t.Fatalf("after = %+v next %q", r.Entries, r.Next)
	}
	if r.Newest == "" || r.Newest == first.Newest {
		t.Errorf("newest did not move: %q", r.Newest)
	}
	// nothing new: no entries, the same newest back
	again := readAll(t, Options{Source: Source{Path: p}, After: r.Newest})
	if len(again.Entries) != 0 || again.Newest != r.Newest || again.Next != "" {
		t.Errorf("quiet poll = %+v newest %q want %q", again.Entries, again.Newest, r.Newest)
	}
}

func TestAfterNeverMissesEntriesWhenMoreThanALimitArrive(t *testing.T) {
	p := copyMain(t)
	mark := readAll(t, Options{Source: Source{Path: p}, Limit: 1}).Newest
	var lines []string
	for i := 0; i < 25; i++ {
		lines = append(lines, reply("2026-10-27T14:02:00Z", "n"+string(rune('a'+i))))
	}
	appendLines(t, p, lines...)
	var got []string
	r := readAll(t, Options{Source: Source{Path: p}, After: mark, Limit: 10})
	for pages := 0; ; pages++ {
		for _, e := range r.Entries {
			got = append(got, e.Text)
		}
		if r.Next == "" {
			break
		}
		if pages > 5 {
			t.Fatal("does not end")
		}
		r = readAll(t, Options{Source: Source{Path: p}, After: mark, Before: r.Next, Limit: 10})
	}
	if len(got) != 25 || got[0] != "ny" || got[24] != "na" {
		t.Errorf("got %d entries %v", len(got), got)
	}
}

func TestAfterGivesALateResultItsToolName(t *testing.T) {
	p := filepath.Join(t.TempDir(), "late.jsonl")
	call := `{"type":"assistant","timestamp":"2026-10-27T14:00:00Z","message":{"content":[{"type":"tool_use","id":"u1","name":"Bash","input":{"command":"sleep 5"}}]}}`
	if err := os.WriteFile(p, []byte(call+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mark := readAll(t, Options{Source: Source{Path: p}}).Newest
	appendLines(t, p, `{"type":"user","timestamp":"2026-10-27T14:00:05Z","message":{"content":[{"type":"tool_result","tool_use_id":"u1","content":"done"}]}}`)
	r := readAll(t, Options{Source: Source{Path: p}, After: mark})
	if len(r.Entries) != 1 || r.Entries[0].Tool != "Bash" || r.Entries[0].DurationMs != 5000 {
		t.Errorf("late result = %+v", r.Entries)
	}
}

func TestAfterAndBeforeBadCursors(t *testing.T) {
	if _, err := Read(Options{Source: Source{Path: "testdata/main.jsonl"}, After: "nope"}); err != ErrBadCursor {
		t.Errorf("err = %v", err)
	}
}

func TestPartialLastLineIsReadOnceItIsWhole(t *testing.T) {
	p := copyMain(t)
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o600)
	whole := reply("2026-10-27T14:03:00Z", "late line")
	f.WriteString(whole[:30])
	f.Close()
	mark := readAll(t, Options{Source: Source{Path: p}}).Newest
	f, _ = os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString(whole[30:] + "\n")
	f.Close()
	r := readAll(t, Options{Source: Source{Path: p}, After: mark})
	if len(r.Entries) != 1 || r.Entries[0].Text != "late line" {
		t.Errorf("entries = %+v", r.Entries)
	}
}
