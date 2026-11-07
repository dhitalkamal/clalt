package copilot

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
)

func eventsPath(f *fixture, id string) string {
	return filepath.Join(f.home, "session-state", id, "events.jsonl")
}

// appendRaw adds raw bytes and keeps mtime moving forward.
func appendRaw(t *testing.T, f *fixture, id, raw string) {
	t.Helper()
	fh, err := os.OpenFile(eventsPath(f, id), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fh.WriteString(raw); err != nil {
		t.Fatal(err)
	}
	fh.Close()
	f.touch(id, time.Second)
}

func TestAnUnchangedFileIsNotReRead(t *testing.T) {
	f := newFixture(t)
	f.session(id1, time.Minute, start(id1, "/p/a"), ev("assistant.turn_start", map[string]any{"p": strings.Repeat("y", 20)}))
	if got := f.list(); got[0].State != feed.StateBusy {
		t.Fatalf("%+v", got)
	}
	// same size, same mtime, different content: only a re-read would notice
	same := start(id1, "/p/a") + "\n" + ev("assistant.turn_end", map[string]any{"p": strings.Repeat("x", 22)}) + "\n"
	old, _ := os.ReadFile(eventsPath(f, id1))
	if len(same) != len(old) {
		t.Fatalf("test setup: %d vs %d", len(same), len(old))
	}
	if err := os.WriteFile(eventsPath(f, id1), []byte(same), 0o644); err != nil {
		t.Fatal(err)
	}
	f.touch(id1, time.Minute)
	if got := f.list(); got[0].State != feed.StateBusy {
		t.Errorf("re-read: %+v", got)
	}
}

func TestAppendedLinesAdvanceTheState(t *testing.T) {
	f := newFixture(t)
	f.session(id1, time.Minute, start(id1, "/p/a"), model("gpt-5"))
	f.list()
	appendRaw(t, f, id1, ev("assistant.turn_start", nil)+"\n")
	if got := f.list(); got[0].State != feed.StateBusy || got[0].Model != "gpt-5" {
		t.Fatalf("%+v", got)
	}
	appendRaw(t, f, id1, ev("assistant.turn_end", nil)+"\n")
	if got := f.list(); got[0].State != feed.StateIdle {
		t.Errorf("%+v", got)
	}
}

func TestAPartialLastLineIsCompletedByTheNextAppend(t *testing.T) {
	f := newFixture(t)
	f.session(id1, time.Minute, start(id1, "/p/a"))
	f.list()
	line := ev("assistant.turn_start", nil)
	appendRaw(t, f, id1, line[:10])
	if got := f.list(); got[0].State != feed.StateIdle {
		t.Fatalf("partial counted: %+v", got)
	}
	appendRaw(t, f, id1, line[10:]+"\n")
	if got := f.list(); got[0].State != feed.StateBusy {
		t.Errorf("%+v", got)
	}
}

func TestALineLargerThanTheTailStillGivesStateAndModel(t *testing.T) {
	f := newFixture(t)
	big := ev("model.messages_snapshot", map[string]any{"messages": strings.Repeat("x", 600<<10)})
	f.session(id1, time.Minute, start(id1, "/p/a"), model("gpt-5"), ev("assistant.turn_start", nil), big)
	got := f.list()
	if len(got) != 1 || got[0].State != feed.StateBusy || got[0].Model != "gpt-5" || got[0].Cwd != "/p/a" {
		t.Errorf("%+v", got)
	}
}

func TestAShrunkOrReplacedFileStartsOver(t *testing.T) {
	f := newFixture(t)
	f.session(id1, time.Minute, start(id1, "/p/a"), model("gpt-5"), ev("assistant.turn_start", nil), ev("hook.start", nil))
	f.list()
	f.session(id1, time.Minute, start(id1, "/p/b"), ev("assistant.turn_end", nil))
	got := f.list()
	if got[0].State != feed.StateIdle || got[0].Model != "" || got[0].Cwd != "/p/b" {
		t.Errorf("%+v", got)
	}
}

func TestAHugeFirstReadFallsBackToTheTail(t *testing.T) {
	f := newFixture(t)
	junk := ev("hook.start", map[string]any{"pad": strings.Repeat("x", 1000)})
	lines := []string{start(id1, "/p/a"), ev("session.shutdown", nil)}
	for i := 0; i < 300; i++ {
		lines = append(lines, junk)
	}
	lines = append(lines, ev("assistant.turn_start", nil))
	f.session(id1, time.Minute, lines...)
	r := f.reader()
	r.FirstReadBytes = 50 << 10
	r.TailBytes = 20 << 10
	got, _ := r.Sessions()
	if len(got) != 1 || got[0].State != feed.StateBusy {
		t.Errorf("%+v", got)
	}
}

func TestCacheEntriesGoWithTheirDirs(t *testing.T) {
	f := newFixture(t)
	f.session(id1, time.Minute, start(id1, "/p/a"))
	f.session("other-2", time.Minute, start("o", "/p/b"))
	f.list()
	if n := len(f.reader().cache); n != 2 {
		t.Fatalf("cache %d", n)
	}
	f.touch("other-2", 7*time.Hour)
	f.list()
	if n := len(f.reader().cache); n != 1 {
		t.Errorf("cache %d", n)
	}
}

func TestConcurrentSessionsCallsAreRaceFree(t *testing.T) {
	f := newFixture(t)
	f.session(id1, time.Minute, start(id1, "/p/a"))
	r := f.reader()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				appendRaw(t, f, id1, ev("assistant.turn_start", nil)+"\n")
			}
			if _, err := r.Sessions(); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
}
