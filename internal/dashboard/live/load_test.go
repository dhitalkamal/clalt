package live

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/usage"
)

func TestTheVisibleAccountsAreListedByLabelWithTheirRecord(t *testing.T) {
	r := newRig(t)
	r.signedIn("work", "w@example.com")
	home := r.signedIn("Home", "h@example.com")
	r.store.SetBudget(home, 10, 50)
	gone := r.signedIn("gone", "g@example.com")
	r.store.Hide(gone)

	snap := r.load(r.adapter())

	if len(snap.Accounts) != 2 || snap.Accounts[0].Label != "Home" || snap.Accounts[1].Label != "work" {
		t.Fatalf("accounts = %+v", snap.Accounts)
	}
	a := snap.Accounts[0]
	if a.ID != home || a.Email != "h@example.com" || a.Org != "Org" || a.Plan != "max plan" || !a.SignedIn || a.Problem != "" {
		t.Errorf("account = %+v", a)
	}
	if a.Budget.Daily != 10 || a.Budget.Weekly != 50 {
		t.Errorf("budget = %+v", a.Budget)
	}
	if !a.Login.SignedInAt.Equal(epoch.Add(-48*time.Hour)) || !a.Login.DueAt.Equal(epoch.Add(-48*time.Hour+loginLife)) {
		t.Errorf("login = %+v", a.Login)
	}
	if !strings.HasSuffix(a.Profile.Folder, filepath.Join("accounts", home)) || a.Profile.ClaudeVersion != "2.1.303" || !a.Profile.SharedSettings || a.Profile.FeedInstalled {
		t.Errorf("profile = %+v", a.Profile)
	}
}

func TestASignedOutAccountKeepsItsEmailAndNoProblem(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("work", "w@example.com")
	r.host.signIns[id] = SignIn{Reason: claudecli.ReasonNotSignedIn, Text: "not signed in"}

	a := byLabel(r.load(r.adapter()), "work")

	if a.SignedIn || a.Problem != "" || a.Email != "w@example.com" || a.Plan != "" || !a.Login.DueAt.IsZero() {
		t.Errorf("account = %+v", a)
	}
}

func TestALoginThatCannotBeUsedIsAProblem(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("work", "w@example.com")
	r.host.signIns[id] = SignIn{Reason: claudecli.ReasonOtherAuthMethod, Text: "signed in with api_key_helper, not a claude.ai account"}

	a := byLabel(r.load(r.adapter()), "work")

	if a.SignedIn || !strings.Contains(a.Problem, "api_key_helper") {
		t.Errorf("account = %+v", a)
	}
}

func TestACheckThatFailsLeavesTheRecordAndSaysSo(t *testing.T) {
	r := newRig(t)
	r.signedIn("work", "w@example.com")
	r.host.checkErr = errors.New("keychain is locked")

	a := byLabel(r.load(r.adapter()), "work")

	if !a.SignedIn || !strings.Contains(a.Problem, "keychain is locked") {
		t.Errorf("account = %+v", a)
	}
}

func TestAnUnusableClaudeIsOneWarningAndNoChecks(t *testing.T) {
	r := newRig(t)
	r.signedIn("a", "a@example.com")
	r.signedIn("b", "b@example.com")
	r.host.verErr = errors.New("claude 2.1.250 is older than the minimum 2.1.301")

	snap := r.load(r.adapter())

	if r.host.checks.Load() != 0 {
		t.Errorf("claude was asked %d times", r.host.checks.Load())
	}
	if len(snap.Warnings) == 0 || !strings.Contains(snap.Warnings[0], "older than the minimum") {
		t.Errorf("warnings = %v", snap.Warnings)
	}
	if !byLabel(snap, "a").SignedIn {
		t.Error("the record still says it is signed in")
	}
}

func TestALoadDoesNotWaitLongerThanTheGraceForAnAnswer(t *testing.T) {
	r := newRig(t)
	r.signedIn("work", "w@example.com")
	r.cfg.Grace = 50 * time.Millisecond
	r.host.gate = make(chan struct{})
	a := r.adapter()

	start := time.Now()
	acc := byLabel(r.load(a), "work")

	if time.Since(start) > 2*time.Second || !acc.SignedIn || acc.Problem != "" {
		t.Errorf("took %v, account = %+v", time.Since(start), acc)
	}
	close(r.host.gate)
}

func TestAnAnswerIsKeptBetweenLoadsAndAsksAtMostFourAtOnce(t *testing.T) {
	r := newRig(t)
	for _, l := range []string{"a", "b", "c", "d", "e", "f"} {
		r.signedIn(l, l+"@example.com")
	}
	r.host.gate = make(chan struct{})
	a := r.adapter()
	r.cfg.Grace = 30 * time.Millisecond
	a.cfg.Grace = 30 * time.Millisecond

	a.Load(context.Background())
	time.Sleep(100 * time.Millisecond)
	if got := r.host.checks.Load(); got != statusWorkers {
		t.Errorf("%d checks are running, the limit is %d", got, statusWorkers)
	}
	close(r.host.gate)
	waitUntil(t, func() bool { return r.host.checks.Load() == 6 })
	time.Sleep(50 * time.Millisecond)
	r.load(a)
	r.load(a)
	if got := r.host.checks.Load(); got != 6 {
		t.Errorf("a fresh answer is not asked for again: %d checks", got)
	}
}

func TestTheStateFileIsTheOneThingThatFailsALoad(t *testing.T) {
	r := newRig(t)
	r.signedIn("work", "w@example.com")
	path := r.store
	_ = path
	if err := os.WriteFile(statePathOf(t), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.adapter().Load(context.Background()); err == nil {
		t.Error("a state file that cannot be read is an error")
	}
}

func TestUsageFillsTheHistoryAndSaysWhatIsNotSplit(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("work", "w@example.com")
	r.host.records = map[string][]usage.Record{
		id:                 {rec(0, "p", "/h/p", "claude-opus-5-5", "s1", 1000, 100)},
		usage.Unattributed: {rec(1, "p", "/h/p", "claude-opus-5-5", "s2", 2000, 100)},
	}

	snap := r.load(r.adapter())

	a := byLabel(snap, "work")
	if len(a.History.Days) != 30 || a.History.Projects[0].Name != "p" || a.LastIn != "~/p" {
		t.Errorf("history = %+v, last in %q", a.History.Projects, a.LastIn)
	}
	if len(snap.Warnings) == 0 || !strings.Contains(snap.Warnings[0], "not split per account") {
		t.Errorf("warnings = %v", snap.Warnings)
	}
}

func TestUsageThatCannotBeReadIsAWarningNotAFailure(t *testing.T) {
	r := newRig(t)
	r.signedIn("work", "w@example.com")
	r.host.usageErr = errors.New("projects leads nowhere")

	snap := r.load(r.adapter())

	if len(snap.Accounts) != 1 || !strings.Contains(strings.Join(snap.Warnings, "|"), "usage could not be read: projects leads nowhere") {
		t.Errorf("snapshot = %+v", snap)
	}
}

func TestTheUsageIsScannedOnceWhileItIsFresh(t *testing.T) {
	r := newRig(t)
	r.signedIn("work", "w@example.com")
	a := r.adapter()
	r.load(a)
	r.load(a)
	r.clock.advance(time.Minute)
	r.load(a)
	waitUntil(t, func() bool { return r.host.scans.Load() == 2 })
	if got := r.host.scans.Load(); got != 2 {
		t.Errorf("%d scans", got)
	}
}

func TestLimitsComeFromTheFeedAndAreMissingWithoutOne(t *testing.T) {
	r := newRig(t)
	r.signedIn("work", "w@example.com")
	if lim := byLabel(r.load(r.adapter()), "work").Limits; !lim.ReadAt.IsZero() {
		t.Errorf("no feed, no reading: %+v", lim)
	}

	r.cfg.Feed = fakeFeed{have: true, installed: true, limits: Limits{ObservedAt: epoch, Five: Window{Percent: 25, ResetsAt: epoch.Add(4 * time.Hour)}}}
	a := byLabel(r.load(r.adapter()), "work")
	if a.Limits.Five.Percent != 25 || a.Limits.Five.Projected != 125 || !a.Profile.FeedInstalled {
		t.Errorf("account = %+v", a)
	}
}

func TestSessionsAreMappedAndCountedByAccount(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("work", "w@example.com")
	r.host.records = map[string][]usage.Record{usage.Unattributed: {rec(0, "p", "/h/p", "claude-opus-5-5", "cs1", 1000000, 1000)}}
	r.cfg.Sessions = &fakeSessions{list: []Session{
		{Handle: "h2", AccountID: id, State: StateNeedsYou, Cwd: "/h/b", Model: "claude-sonnet-5", StartedAt: epoch},
		{Handle: "h1", AccountID: id, State: StateBusy, Cwd: "/h/a", ClaudeSessionID: "cs1", StartedAt: epoch.Add(-time.Hour)},
		{Handle: "h3", AccountID: id, State: StateEnded},
		{Handle: "h4", AccountID: "nobody", State: StateIdle},
	}}

	snap := r.load(r.adapter())

	ss := byLabel(snap, "work").Sessions
	if len(ss) != 2 || ss[0].ID != "h1" || ss[0].State != "working" || ss[0].Folder != "~/a" || ss[0].Cost <= 0 ||
		ss[1].State != "waiting" || ss[1].Model != "Sonnet 5" {
		t.Errorf("sessions = %+v", ss)
	}
	for _, w := range snap.Warnings {
		if strings.Contains(w, "not shown") {
			t.Errorf("a real registry needs no warning: %v", snap.Warnings)
		}
	}
}

func TestWithoutARegistrySessionsAreSaidToBeMissing(t *testing.T) {
	r := newRig(t)
	r.signedIn("work", "w@example.com")
	snap := r.load(r.adapter())
	if !strings.Contains(strings.Join(snap.Warnings, "|"), "running sessions are not shown yet") {
		t.Errorf("warnings = %v", snap.Warnings)
	}
}

func TestAFailingRegistryIsAWarning(t *testing.T) {
	r := newRig(t)
	r.signedIn("work", "w@example.com")
	r.cfg.Sessions = &fakeSessions{err: errors.New("registry locked")}
	all := strings.Join(r.load(r.adapter()).Warnings, "|")
	for _, want := range []string{"registry locked"} {
		if !strings.Contains(all, want) {
			t.Errorf("warnings %q lack %q", all, want)
		}
	}
}

func TestAnIdThatCannotBeAFolderIsAProblemAndNeverAsked(t *testing.T) {
	r := newRig(t)
	writeState(t, `{"accounts":{"a/b":{"label":"odd","email":"o@example.com","signedInAt":"2026-01-01T00:00:00Z","addedAt":"2026-01-01T00:00:00Z"}}}`)

	a := byLabel(r.load(r.adapter()), "odd")

	if a.SignedIn || a.Problem == "" || r.host.checks.Load() != 0 {
		t.Errorf("account = %+v, checks %d", a, r.host.checks.Load())
	}
}

func TestLinksThatCannotBeCheckedAreAWarningNotAClaim(t *testing.T) {
	r := newRig(t)
	r.signedIn("work", "w@example.com")
	r.host.linksErr = errors.New("permission denied")

	snap := r.load(r.adapter())

	if byLabel(snap, "work").Profile.SharedSettings || !strings.Contains(strings.Join(snap.Warnings, "|"), "could not check the links of work: permission denied") {
		t.Errorf("snapshot = %+v", snap)
	}
}
