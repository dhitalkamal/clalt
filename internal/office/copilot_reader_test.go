package office

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/copilot"
	"github.com/dhitalkamal/clalt/internal/feed"
)

type fakeCopilot []copilot.Session

func (f fakeCopilot) Sessions() ([]copilot.Session, error) { return f, nil }

func TestReaderSessionsShowInTheCopilotTeam(t *testing.T) {
	f := newFixture(t)
	h := copilotHandle(t, "aaaa-1111")
	f.cp = fakeCopilot{{ID: "aaaa-1111", Handle: h, Cwd: "/p/site", Model: "gpt-5", State: feed.StateBusy, StartedAt: t0.Add(-time.Hour), UpdatedAt: t0}}

	snap := f.build()

	if len(snap.Sessions) != 1 {
		t.Fatalf("%+v", snap.Sessions)
	}
	s := snap.Sessions[0]
	if s.Handle != h || s.Account != CopilotAccount || s.Tool != ToolCopilot || s.State != "busy" || s.Project != "site" || s.Model != "gpt-5" {
		t.Errorf("%+v", s)
	}
	if got := snap.Accounts[len(snap.Accounts)-1]; got.ID != CopilotAccount {
		t.Errorf("%+v", snap.Accounts)
	}
}

func TestTheFresherOfFeedAndReaderWinsAndIsListedOnce(t *testing.T) {
	h := copilotHandle(t, "aaaa-1111")
	for name, c := range map[string]struct {
		feedAge, readAge time.Duration
		want             string
	}{
		"feed fresher":   {time.Minute, 5 * time.Minute, "idle"},
		"reader fresher": {5 * time.Minute, time.Minute, "busy"},
	} {
		f := newFixture(t)
		f.state(h, "", feed.StateIdle, t0.Add(-c.feedAge), nil)
		f.cp = fakeCopilot{{ID: "aaaa-1111", Handle: h, State: feed.StateBusy, UpdatedAt: t0.Add(-c.readAge)}}
		snap := f.build()
		if len(snap.Sessions) != 1 || snap.Sessions[0].State != c.want {
			t.Errorf("%s: %+v", name, snap.Sessions)
		}
	}
}

func TestARealReaderOverFilesMakesTheCopilotTeam(t *testing.T) {
	home := t.TempDir()
	id := "5f2c951e-6080-437c-a352-c92e842ac58c"
	dir := filepath.Join(home, "session-state", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines := `{"type":"session.start","data":{"sessionId":"` + id + `","startTime":"2026-11-07T17:00:00.000Z","context":{"cwd":"/p/site"}}}` + "\n" +
		`{"type":"assistant.turn_start","data":{}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	f.cp = &copilot.Reader{Home: home, Now: func() time.Time { return time.Now() }}

	snap := f.build()

	if len(snap.Sessions) != 1 || snap.Sessions[0].Tool != ToolCopilot || snap.Sessions[0].State != "busy" {
		t.Fatalf("%+v", snap.Sessions)
	}
	if got := snap.Accounts[len(snap.Accounts)-1]; got.ID != CopilotAccount || got.Label != "Copilot" {
		t.Errorf("%+v", snap.Accounts)
	}
}
