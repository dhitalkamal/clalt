package copilot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
)

const id1 = "5f2c951e-6080-437c-a352-c92e842ac58c"

func TestAnOpenSessionIsIdleAfterTurnEnd(t *testing.T) {
	f := newFixture(t)
	f.session(id1, time.Minute, start(id1, "/p/a"), prompt(), ev("assistant.turn_start", nil), ev("assistant.turn_end", nil), ev("session.usage_checkpoint", nil))
	got := f.list()
	if len(got) != 1 || got[0].State != feed.StateIdle || got[0].Handle != "cp-"+id1 || got[0].ID != id1 {
		t.Fatalf("%+v", got)
	}
}

func TestASessionIsBusyAfterTurnStartOrPrompt(t *testing.T) {
	for _, last := range []string{"assistant.turn_start", "user.message", "model.turn_started"} {
		f := newFixture(t)
		f.session(id1, time.Minute, start(id1, "/p/a"), ev("assistant.turn_end", nil), ev(last, nil), ev("hook.start", nil))
		got := f.list()
		if len(got) != 1 || got[0].State != feed.StateBusy {
			t.Errorf("%s: %+v", last, got)
		}
	}
}

func TestASessionWithOnlyStartIsIdle(t *testing.T) {
	f := newFixture(t)
	f.session(id1, time.Minute, start(id1, "/p/a"), ev("hook.start", nil))
	if got := f.list(); len(got) != 1 || got[0].State != feed.StateIdle {
		t.Errorf("%+v", got)
	}
}

func TestUnknownEventsChangeNothing(t *testing.T) {
	f := newFixture(t)
	f.session(id1, time.Minute, start(id1, "/p/a"), ev("assistant.turn_start", nil), ev("tool.something_new", nil))
	if got := f.list(); len(got) != 1 || got[0].State != feed.StateBusy {
		t.Errorf("%+v", got)
	}
}

func TestAShutdownSessionIsNotListed(t *testing.T) {
	f := newFixture(t)
	f.session(id1, time.Minute, start(id1, "/p/a"), ev("assistant.turn_end", nil), ev("session.shutdown", nil))
	if got := f.list(); len(got) != 0 {
		t.Errorf("%+v", got)
	}
}

func TestAStaleSessionIsNotListed(t *testing.T) {
	f := newFixture(t)
	f.session(id1, 7*time.Hour, start(id1, "/p/a"), ev("assistant.turn_end", nil))
	if got := f.list(); len(got) != 0 {
		t.Errorf("%+v", got)
	}
}

func TestADirWithoutEventsOrWithABadNameIsSkipped(t *testing.T) {
	f := newFixture(t)
	f.session("bad name", time.Minute, start(id1, "/p/a"))
	f.session("..x.y", time.Minute, start(id1, "/p/a"))
	if err := os.MkdirAll(filepath.Join(f.home, "session-state", "empty-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.home, "session-state", "afile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := f.list(); len(got) != 0 {
		t.Errorf("%+v", got)
	}
}

func TestNoSessionStateFolderIsNoSessions(t *testing.T) {
	if got := newFixture(t).list(); len(got) != 0 {
		t.Errorf("%+v", got)
	}
}

func TestATruncatedLastLineIsIgnored(t *testing.T) {
	f := newFixture(t)
	dir := f.session(id1, time.Minute, start(id1, "/p/a"), ev("assistant.turn_start", nil))
	fh, _ := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_APPEND|os.O_WRONLY, 0)
	fmt.Fprint(fh, `{"type":"assistant.turn_end","da`)
	fh.Close()
	f.touch(id1, time.Minute)
	if got := f.list(); len(got) != 1 || got[0].State != feed.StateBusy {
		t.Errorf("%+v", got)
	}
}

func TestAHugeFileIsOnlyReadFromTheTail(t *testing.T) {
	f := newFixture(t)
	junk := ev("hook.start", map[string]any{"pad": strings.Repeat("x", 1000)})
	lines := []string{start(id1, "/p/a"), ev("session.shutdown", nil)}
	for i := 0; i < 2000; i++ {
		lines = append(lines, junk)
	}
	lines = append(lines, ev("assistant.turn_start", nil))
	f.session(id1, time.Minute, lines...)
	got := f.list()
	// the head (start, shutdown) is far outside the tail window
	if len(got) != 1 || got[0].State != feed.StateBusy {
		t.Fatalf("%+v", got)
	}
}

func TestCwdModelAndStartFromTheFiles(t *testing.T) {
	f := newFixture(t)
	dir := f.session(id1, time.Minute, start(id1, "/from/events"), model("auto"), model("gpt-5"), ev("assistant.turn_end", nil))
	got := f.list()
	if got[0].Cwd != "/from/events" || got[0].Model != "gpt-5" || !got[0].StartedAt.Equal(time.Date(2026, 11, 7, 17, 0, 0, 0, time.UTC)) {
		t.Fatalf("%+v", got[0])
	}
	if !got[0].UpdatedAt.Equal(now.Add(-time.Minute)) {
		t.Errorf("updated %v", got[0].UpdatedAt)
	}
	ws := "id: " + id1 + "\ncwd: /from/workspace\ncreated_at: 2026-11-07T16:00:00.000Z\nsummary: " + marker + "\n"
	if err := os.WriteFile(filepath.Join(dir, "workspace.yaml"), []byte(ws), 0o644); err != nil {
		t.Fatal(err)
	}
	got = f.list()
	if got[0].Cwd != "/from/workspace" {
		t.Errorf("cwd %q", got[0].Cwd)
	}
}

func TestStartFallsBackToWorkspaceWhenTheStartEventIsOutOfTail(t *testing.T) {
	f := newFixture(t)
	dir := f.session(id1, time.Minute, ev("assistant.turn_end", nil))
	ws := "id: " + id1 + "\ncwd: /w\ncreated_at: 2026-11-07T16:00:00.000Z\n"
	_ = os.WriteFile(filepath.Join(dir, "workspace.yaml"), []byte(ws), 0o644)
	got := f.list()
	if !got[0].StartedAt.Equal(time.Date(2026, 11, 7, 16, 0, 0, 0, time.UTC)) {
		t.Errorf("%+v", got[0])
	}
}

func TestNoFreeTextComesBack(t *testing.T) {
	f := newFixture(t)
	dir := f.session(id1, time.Minute, start(id1, "/p/a"),
		ev("hook.start", map[string]any{"hookType": "userPromptSubmitted", "input": map[string]any{"prompt": marker, "initialPrompt": marker}}),
		prompt(), ev("instruction.update", map[string]any{"content": marker}), ev("assistant.message", map[string]any{"content": marker}), ev("assistant.turn_start", nil))
	_ = os.WriteFile(filepath.Join(dir, "workspace.yaml"), []byte("cwd: /p/a\nname: "+marker+"\nsummary: "+marker+"\n"), 0o644)
	if s := fmt.Sprintf("%+v", f.list()); strings.Contains(s, marker) {
		t.Errorf("leaked: %s", s)
	}
}

func TestOtherFilesInTheHomeAreNeverOpened(t *testing.T) {
	f := newFixture(t)
	f.session(id1, time.Minute, start(id1, "/p/a"))
	// unreadable decoys: opening any of them would error the run
	for _, n := range []string{"config.json", "providers.json", "mcp-config.json", "permissions-config.json"} {
		if err := os.WriteFile(filepath.Join(f.home, n), []byte("x"), 0); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.list(); len(got) != 1 {
		t.Errorf("%+v", got)
	}
}

func TestNewestDirsAreKeptWhenThereAreTooMany(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 5; i++ {
		f.session(fmt.Sprintf("s-%d", i), time.Duration(i+1)*time.Minute, start("s", "/p"))
	}
	r := f.reader()
	r.MaxDirs = 2
	got, _ := r.Sessions()
	if len(got) != 2 || got[0].ID != "s-0" || got[1].ID != "s-1" {
		t.Errorf("%+v", got)
	}
}

func TestACopilotHomeEnvIsRespected(t *testing.T) {
	t.Setenv("COPILOT_HOME", "/x/cp")
	if Home() != "/x/cp" {
		t.Errorf("%q", Home())
	}
	t.Setenv("COPILOT_HOME", "")
	t.Setenv("HOME", "/h")
	if Home() != "/h/.copilot" {
		t.Errorf("%q", Home())
	}
}
