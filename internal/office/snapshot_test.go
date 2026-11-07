package office

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

var t0 = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

const (
	runHandle = "0123456789abcdef"
	deadHand  = "fedcba9876543210"
)

// fixture is a registry, a feed and an account list inside a temp dir.
type fixture struct {
	t    *testing.T
	reg  registry.Registry
	feed feed.Store
	accs []Account
	cp   CopilotSource
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	return &fixture{
		t:    t,
		reg:  registry.Registry{Dir: filepath.Join(root, "sessions")},
		feed: feed.Store{Dir: filepath.Join(root, "feed")},
		accs: []Account{{ID: "work", Label: "work"}, {ID: "personal", Label: "personal"}},
	}
}

func (f *fixture) record(handle, account, cwd string, pid int) {
	f.t.Helper()
	err := f.reg.Create(registry.Record{Handle: handle, Account: account, Cwd: cwd, WrapperPID: pid, PID: pid, StartedAt: t0.Add(-time.Hour)})
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) state(handle, account string, st feed.State, at time.Time, fn func(*feed.SessionState)) {
	f.t.Helper()
	err := f.feed.UpdateSession(handle, account, at, func(s *feed.SessionState) bool {
		s.State = st
		if fn != nil {
			fn(s)
		}
		return true
	})
	if err != nil {
		f.t.Fatal(err)
	}
}

// subagents sets the subagents of a session in its feed file.
func (f *fixture) subagents(handle, account string, subs ...feed.Subagent) {
	f.t.Helper()
	f.state(handle, account, feed.StateBusy, t0, func(s *feed.SessionState) { s.Subagents = subs })
}

func (f *fixture) build() Snapshot {
	f.t.Helper()
	snap, err := Build(RealSources(Real{
		Registry: f.reg, Feed: f.feed, Accounts: staticAccounts(f.accs), Copilot: f.cp, Now: func() time.Time { return t0 },
	}))
	if err != nil {
		f.t.Fatal(err)
	}
	return snap
}

type staticAccounts []Account

func (s staticAccounts) Accounts() ([]Account, error) { return s, nil }

func TestSnapshotHasTheVersionAndEmptyListsWhenNothingRuns(t *testing.T) {
	snap := newFixture(t).build()
	raw, _ := json.Marshal(snap)
	if snap.V != 1 {
		t.Fatalf("v = %d, want 1", snap.V)
	}
	if !strings.Contains(string(raw), `"sessions":[]`) {
		t.Errorf("sessions must encode as [] and not null: %s", raw)
	}
}

func TestSnapshotShowsARunningSessionWithItsFeedState(t *testing.T) {
	f := newFixture(t)
	f.record(runHandle, "work", "/home/x/Projects/clalt", os.Getpid())
	f.state(runHandle, "work", feed.StateNeedsYou, t0, func(s *feed.SessionState) {
		s.Reason, s.Model, s.Cwd = "permission_prompt", "claude-opus-5-5", "/home/x/Projects/clalt"
	})
	snap := f.build()
	if len(snap.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(snap.Sessions))
	}
	s := snap.Sessions[0]
	if s.Handle != runHandle || s.Account != "work" || s.Project != "clalt" || s.State != "needs-you" ||
		s.NeedsYouReason != "permission_prompt" || s.Model != "claude-opus-5-5" {
		t.Errorf("session = %+v", s)
	}
	if !s.StartedAt.Equal(t0.Add(-time.Hour)) {
		t.Errorf("startedAt = %v", s.StartedAt)
	}
}

func TestSessionWithNoFeedFileIsStartingAndAReasonOnlyShowsWhenNeedsYou(t *testing.T) {
	f := newFixture(t)
	f.record(runHandle, "work", "/p/app", os.Getpid())
	if got := f.build().Sessions[0]; got.State != "starting" || got.NeedsYouReason != "" || got.Project != "app" {
		t.Errorf("session = %+v", got)
	}
	f.state(runHandle, "work", feed.StateBusy, t0, func(s *feed.SessionState) { s.Reason = "permission_prompt" })
	if got := f.build().Sessions[0]; got.NeedsYouReason != "" {
		t.Errorf("a busy session must not carry a reason: %+v", got)
	}
}

func TestSessionWhoseWrapperIsGoneIsLeftOut(t *testing.T) {
	f := newFixture(t)
	f.record(deadHand, "work", "/p/gone", 999999999)
	if n := len(f.build().Sessions); n != 0 {
		t.Errorf("sessions = %d, want 0", n)
	}
}

func TestPlainClaudeSessionsAreIncludedWhileFreshAndNotEnded(t *testing.T) {
	f := newFixture(t)
	f.state("ext-aaaa-1111", "personal", feed.StateIdle, t0.Add(-time.Minute), func(s *feed.SessionState) { s.Cwd = "/p/site" })
	f.state("ext-bbbb-2222", "personal", feed.StateEnded, t0.Add(-time.Minute), nil)
	f.state("ext-cccc-3333", "personal", feed.StateBusy, t0.Add(-7*time.Hour), nil)
	snap := f.build()
	if len(snap.Sessions) != 1 || snap.Sessions[0].Handle != "ext-aaaa-1111" || snap.Sessions[0].Project != "site" {
		t.Fatalf("sessions = %+v", snap.Sessions)
	}
}

func TestLimitsAreNullWhenUnknownAndFilledWhenKnown(t *testing.T) {
	f := newFixture(t)
	reset := t0.Add(2 * time.Hour)
	if err := f.feed.MergeLimits("work", &feed.Window{UsedPct: 41.5, ResetsAt: reset}, nil, t0); err != nil {
		t.Fatal(err)
	}
	snap := f.build()
	by := map[string]AccountOut{}
	for _, a := range snap.Accounts {
		by[a.ID] = a
	}
	w := by["work"].Limits
	if w.FiveHour == nil || w.FiveHour.UsedPct != 41.5 || !w.FiveHour.ResetsAt.Equal(reset) {
		t.Errorf("work five hour = %+v", w.FiveHour)
	}
	if w.SevenDay != nil || w.ReadAt == nil {
		t.Errorf("work seven day must be null (unknown, not 0) and readAt set: %+v", w)
	}
	p := by["personal"].Limits
	if p.FiveHour != nil || p.SevenDay != nil || p.ReadAt != nil {
		t.Errorf("personal has no reading at all: %+v", p)
	}
	raw, _ := json.Marshal(snap)
	if !strings.Contains(string(raw), `"sevenDay":null`) || !strings.Contains(string(raw), `"readAt":null`) {
		t.Errorf("unknown must encode as null: %s", raw)
	}
}

func TestAWindowThatHasResetIsUnknown(t *testing.T) {
	f := newFixture(t)
	if err := f.feed.MergeLimits("work", &feed.Window{UsedPct: 90, ResetsAt: t0.Add(-time.Minute)}, nil, t0.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	for _, a := range f.build().Accounts {
		if a.ID == "work" && a.Limits.FiveHour != nil {
			t.Errorf("a window that reset since the reading must not show the old figure: %+v", a.Limits.FiveHour)
		}
	}
}

func TestSubagentsComeFromTheFeedStateAndAreOrdered(t *testing.T) {
	f := newFixture(t)
	f.record(runHandle, "work", "/p/app", os.Getpid())
	end := t0.Add(-time.Minute)
	f.subagents(runHandle, "work",
		feed.Subagent{ID: "b", Type: "Plan", Label: "plan it", State: "running", StartedAt: t0.Add(-2 * time.Minute)},
		feed.Subagent{ID: "a", Type: "Explore", Label: "map it", State: "done", StartedAt: t0.Add(-5 * time.Minute), EndedAt: &end})
	subs := f.build().Sessions[0].Subagents
	if len(subs) != 2 || subs[0].ID != "a" || subs[0].EndedAt == nil || subs[1].EndedAt != nil || subs[1].Type != "Plan" {
		t.Errorf("subagents = %+v", subs)
	}
}

func TestSessionWithNoSubagentsEncodesAnEmptyList(t *testing.T) {
	f := newFixture(t)
	f.record(runHandle, "work", "/p/app", os.Getpid())
	raw, _ := json.Marshal(f.build())
	if !strings.Contains(string(raw), `"subagents":[]`) {
		t.Errorf("want an empty list, not null: %s", raw)
	}
}

func TestPlainClaudeSessionsCarryTheirSubagentsToo(t *testing.T) {
	f := newFixture(t)
	f.state("ext-aaaa-1111", "personal", feed.StateBusy, t0, func(s *feed.SessionState) {
		s.Subagents = []feed.Subagent{{ID: "x", Type: "Explore", State: "running", StartedAt: t0}}
	})
	if subs := f.build().Sessions[0].Subagents; len(subs) != 1 || subs[0].ID != "x" {
		t.Errorf("subagents = %+v", subs)
	}
}

func TestAtMostFiftySubagentsPerSessionReachTheSnapshot(t *testing.T) {
	f := newFixture(t)
	f.record(runHandle, "work", "/p/app", os.Getpid())
	var many []feed.Subagent
	for i := 0; i < 80; i++ {
		many = append(many, feed.Subagent{ID: fmt.Sprintf("a%02d", i), Type: "x", State: "running", StartedAt: t0})
	}
	f.subagents(runHandle, "work", many...)
	if n := len(f.build().Sessions[0].Subagents); n != maxSubagentsPerSession {
		t.Errorf("got %d, want %d", n, maxSubagentsPerSession)
	}
}

func TestEveryStringOfTheSnapshotIsCleanedAndKeepsOnlyTheFolderName(t *testing.T) {
	f := newFixture(t)
	f.accs = []Account{{ID: "work", Label: "wo\x1b[31mrk\n"}}
	f.record(runHandle, "work", "/home/x/\x1b]0;pwned\x07evil\u202eproj", os.Getpid())
	f.subagents(runHandle, "work", feed.Subagent{ID: "a", Type: "Exp\x00lore", Label: strings.Repeat("x", 500), State: "running", StartedAt: t0})
	snap := f.build()
	raw, _ := json.Marshal(snap)
	for _, bad := range []string{"\\u001b", "\\u0007", "\\u202e", "\\u0000", "\\n", "/home/x"} {
		if strings.Contains(string(raw), bad) {
			t.Errorf("snapshot contains %q: %s", bad, raw)
		}
	}
	if l := len(snap.Sessions[0].Subagents[0].Label); l > 80 {
		t.Errorf("label not capped: %d", l)
	}
}

func TestSnapshotCarriesNoEmailOrSecretsFields(t *testing.T) {
	f := newFixture(t)
	f.record(runHandle, "work", "/p/app", os.Getpid())
	raw, _ := json.Marshal(f.build())
	for _, banned := range []string{"email", "token", "env", "prompt", "transcript", "cwd", "folder"} {
		if strings.Contains(strings.ToLower(string(raw)), banned) {
			t.Errorf("snapshot mentions %q: %s", banned, raw)
		}
	}
}

func TestAnEmailLookingLabelOrIdIsCutAtTheAt(t *testing.T) {
	f := newFixture(t)
	f.accs = []Account{{ID: "me@example.com", Label: "me@example.com"}, {ID: "x", Label: "@corp"}}
	f.record(runHandle, "me@example.com", "/p/app", os.Getpid())
	snap := f.build()
	raw, _ := json.Marshal(snap)
	if strings.Contains(string(raw), "@") || strings.Contains(string(raw), "example.com") {
		t.Errorf("snapshot has an email-looking value: %s", raw)
	}
	labels := map[string]bool{}
	for _, a := range snap.Accounts {
		labels[a.Label] = true
	}
	if !labels["me"] || !labels["account"] || snap.Sessions[0].Account != "me" {
		t.Errorf("accounts %+v session account %q", snap.Accounts, snap.Sessions[0].Account)
	}
}
