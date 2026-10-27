package cli

import (
	"os"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
	"github.com/dhitalkamal/clalt/internal/wrapper"
)

const adapterHandle = "0123456789abcdef"

func adapterRegistry(t *testing.T) (registryAdapter, registry.Registry, feed.Store) {
	t.Helper()
	reg := registry.Registry{Dir: t.TempDir()}
	store := feed.Store{Dir: t.TempDir()}
	err := reg.Create(registry.Record{
		Handle: adapterHandle, Account: "a", Folder: "/fa", WrapperPID: os.Getpid(), PID: 11,
		ClaudeSessionID: "sess-1", Cwd: "/w", Terminal: registry.Terminal{Kind: "kitty"}, StartedAt: theClock,
	})
	if err != nil {
		t.Fatal(err)
	}
	return registryAdapter{reg: reg, store: store, now: func() time.Time { return theClock }}, reg, store
}

func TestRegistryAdapterListsRecordsInTheWrappersTerms(t *testing.T) {
	ad, reg, _ := adapterRegistry(t)

	recs, err := ad.List()
	if err != nil || len(recs) != 1 {
		t.Fatalf("list: %v, %v", recs, err)
	}
	r := recs[0]
	if r.Handle != adapterHandle || r.Account != "a" || r.ClaudePID != 11 || r.ClaudeSessionID != "sess-1" ||
		r.Cwd != "/w" || r.Terminal != "kitty" || r.WrapperPID != os.Getpid() {
		t.Errorf("record = %+v", r)
	}
	if ad.Dir() != reg.Dir {
		t.Errorf("dir = %q, want %q", ad.Dir(), reg.Dir)
	}
}

func TestRegistryAdapterUpdateChangesOnlyWhatIsSet(t *testing.T) {
	ad, reg, _ := adapterRegistry(t)

	if err := ad.Update(adapterHandle, wrapper.Change{ClaudePID: 42}); err != nil {
		t.Fatal(err)
	}
	rec, _, _ := reg.Get(adapterHandle)
	if rec.PID != 42 || rec.Account != "a" || rec.Folder != "/fa" {
		t.Errorf("after a pid change: %+v", rec)
	}

	if err := ad.Update(adapterHandle, wrapper.Change{Account: "b", Folder: "/fb", ClaudePID: 43}); err != nil {
		t.Fatal(err)
	}
	rec, _, _ = reg.Get(adapterHandle)
	if rec.PID != 43 || rec.Account != "b" || rec.Folder != "/fb" {
		t.Errorf("after a move: %+v", rec)
	}
}

// the new claude has said nothing yet. leaving the old idle state would make the
// wrapper read it as a pause of the claude it just started, and move again
func TestRegistryAdapterMoveResetsTheFeedToStarting(t *testing.T) {
	ad, _, store := adapterRegistry(t)
	err := store.UpdateSession(adapterHandle, "a", theClock.Add(-time.Hour), func(s *feed.SessionState) bool {
		s.State = feed.StateIdle
		return true
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := ad.Update(adapterHandle, wrapper.Change{Account: "b", Folder: "/fb", ClaudePID: 5}); err != nil {
		t.Fatal(err)
	}

	st, _ := store.Session(adapterHandle)
	if st.State != feed.StateStarting || st.Account != "b" {
		t.Errorf("feed state = %s as %s, want starting as b", st.State, st.Account)
	}
}

func TestFeedAdapterSessionMapsStateAndLimitHit(t *testing.T) {
	store := feed.Store{Dir: t.TempDir()}
	ad := feedAdapter{store: store}
	set := func(state feed.State, lastErr string) {
		t.Helper()
		err := store.UpdateSession(adapterHandle, "a", theClock, func(s *feed.SessionState) bool {
			s.State, s.LastError, s.Cwd = state, lastErr, "/w"
			return true
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		state   feed.State
		lastErr string
		want    wrapper.State
		limit   bool
	}{
		{feed.StateStarting, "", wrapper.Starting, false},
		{feed.StateBusy, "", wrapper.Busy, false},
		{feed.StateIdle, "", wrapper.Idle, false},
		{feed.StateIdle, "rate_limit", wrapper.Idle, true},
		{feed.StateIdle, "server_error", wrapper.Idle, false},
		{feed.StateNeedsYou, "", wrapper.NeedsYou, false},
		{feed.StateEnded, "", wrapper.Ended, false},
	}
	for _, c := range cases {
		set(c.state, c.lastErr)
		info, ok := ad.Session(adapterHandle)
		if !ok || info.State != c.want || info.LimitHit != c.limit || info.Cwd != "/w" || !info.UpdatedAt.Equal(theClock) {
			t.Errorf("%s/%q: %+v ok=%v, want state %v limit %v", c.state, c.lastErr, info, ok, c.want, c.limit)
		}
	}
	if _, ok := ad.Session("fedcba9876543210"); ok {
		t.Error("a session with no file was found")
	}
}

func TestFeedAdapterLimitsAreTheFeedsWindows(t *testing.T) {
	store := feed.Store{Dir: t.TempDir()}
	ad := feedAdapter{store: store}
	five := &feed.Window{UsedPct: 40, ResetsAt: time.Now().Add(2 * time.Hour)}
	if err := store.MergeLimits("a", five, nil, time.Now()); err != nil {
		t.Fatal(err)
	}

	l, ok := ad.Limits("a")
	if !ok || !l.FiveHour.Known || l.FiveHour.Used != 40 || !l.FiveHour.ResetsAt.Equal(five.ResetsAt) || l.SevenDay.Known {
		t.Errorf("limits = %+v ok=%v", l, ok)
	}
	if l.ObservedAt.IsZero() {
		t.Error("observedAt is not set, so the limits would never count as recent")
	}
	if _, ok := ad.Limits("nobody"); ok {
		t.Error("limits for an account the feed never saw")
	}
}

// the picker reads the limits the hooks wrote, with no fake in between
func TestRunWithoutANamePicksByTheLimitsInTheRealFeed(t *testing.T) {
	e := newEnv(t)
	e.signIn("alpha", "a@example.com")
	e.signIn("beta", "b@example.com")
	e.at(theClock)
	store := feed.Store{Dir: config.FeedDir()}
	seen := func(label string, used float64) {
		t.Helper()
		w := &feed.Window{UsedPct: used, ResetsAt: theClock.Add(2 * time.Hour)}
		if err := store.MergeLimits(e.account(label).Name, w, nil, theClock.Add(-time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	seen("alpha", 80)
	seen("beta", 10)

	if code := e.run("run"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := ranAs(e); got != e.folder("beta") {
		t.Errorf("ran as %q, want beta at %q: %s", got, e.folder("beta"), e.errOut.String())
	}
}
