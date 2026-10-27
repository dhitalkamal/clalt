package garden

import (
	"sync"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

var t0 = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func accts(ids ...string) []Account {
	var out []Account
	for _, id := range ids {
		out = append(out, Account{ID: id, Label: id})
	}
	return out
}

func needs(cabin map[string]int, ws, rs int) Needs {
	return Needs{CabinDesks: cabin, WorkspaceDesks: ws, ResearchDesks: rs}
}

// wsDesks is the workspace desks of the layout the tracker serves now.
func wsDesks(t *testing.T, tr *Tracker) int {
	t.Helper()
	l, _, err := tr.Layout("rows")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, p := range l.Pavilions {
		if p.Type == "workspace" {
			n += Desks(p)
		}
	}
	return n
}

func cabinCount(t *testing.T, tr *Tracker) int {
	t.Helper()
	l, _, err := tr.Layout("rows")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, p := range l.Pavilions {
		if p.Type == "cabin" {
			n++
		}
	}
	return n
}

func TestTrackerFirstObserveApplies(t *testing.T) {
	tr := NewTracker()
	tr.Observe(t0, accts("a"), needs(nil, 24, 4))
	if got := wsDesks(t, tr); got != 24 {
		t.Errorf("first observe: %d workspace desks, want 24", got)
	}
}

func TestTrackerGrowsAfterThirtySeconds(t *testing.T) {
	tr := NewTracker()
	tr.Observe(t0, accts("a"), needs(nil, 8, 4))
	tr.Observe(t0.Add(time.Second), accts("a"), needs(nil, 24, 4))
	tr.Observe(t0.Add(30*time.Second), accts("a"), needs(nil, 24, 4))
	if got := wsDesks(t, tr); got != 8 {
		t.Errorf("29 s in: %d desks, want 8", got)
	}
	tr.Observe(t0.Add(31*time.Second), accts("a"), needs(nil, 24, 4))
	if got := wsDesks(t, tr); got != 24 {
		t.Errorf("30 s held: %d desks, want 24", got)
	}
}

func TestTrackerShortSpikeIsIgnored(t *testing.T) {
	tr := NewTracker()
	tr.Observe(t0, accts("a"), needs(nil, 8, 4))
	tr.Observe(t0.Add(time.Second), accts("a"), needs(nil, 24, 4))
	tr.Observe(t0.Add(20*time.Second), accts("a"), needs(nil, 8, 4))
	tr.Observe(t0.Add(2*time.Minute), accts("a"), needs(nil, 8, 4))
	if got := wsDesks(t, tr); got != 8 {
		t.Errorf("after a spike: %d desks, want 8", got)
	}
}

func TestTrackerGrowTakesTheLowestNeedOfTheWindow(t *testing.T) {
	tr := NewTracker()
	tr.Observe(t0, accts("a"), needs(nil, 8, 4))
	tr.Observe(t0.Add(time.Second), accts("a"), needs(nil, 40, 4))
	tr.Observe(t0.Add(10*time.Second), accts("a"), needs(nil, 16, 4))
	tr.Observe(t0.Add(40*time.Second), accts("a"), needs(nil, 40, 4))
	if got := wsDesks(t, tr); got != 16 {
		t.Errorf("got %d desks, want 16 (held the whole window)", got)
	}
}

func TestTrackerShrinksAfterTenMinutes(t *testing.T) {
	tr := NewTracker()
	tr.Observe(t0, accts("a"), needs(nil, 24, 4))
	tr.Observe(t0.Add(time.Second), accts("a"), needs(nil, 8, 4))
	tr.Observe(t0.Add(10*time.Minute), accts("a"), needs(nil, 8, 4))
	if got := wsDesks(t, tr); got != 24 {
		t.Errorf("9m59 in: %d desks, want 24", got)
	}
	tr.Observe(t0.Add(10*time.Minute+2*time.Second), accts("a"), needs(nil, 8, 4))
	if got := wsDesks(t, tr); got != 8 {
		t.Errorf("10 min held: %d desks, want 8", got)
	}
}

func TestTrackerDirectionFlipRestartsTheHold(t *testing.T) {
	tr := NewTracker()
	tr.Observe(t0, accts("a"), needs(nil, 16, 4))
	tr.Observe(t0.Add(time.Second), accts("a"), needs(nil, 24, 4))
	tr.Observe(t0.Add(25*time.Second), accts("a"), needs(nil, 8, 4))
	tr.Observe(t0.Add(26*time.Second), accts("a"), needs(nil, 24, 4))
	tr.Observe(t0.Add(40*time.Second), accts("a"), needs(nil, 24, 4))
	if got := wsDesks(t, tr); got != 16 {
		t.Errorf("got %d desks, want 16 (grow hold restarted at 26 s)", got)
	}
}

func TestTrackerAccountsApplyAtOnce(t *testing.T) {
	tr := NewTracker()
	tr.Observe(t0, accts("a"), needs(nil, 8, 4))
	if cabinCount(t, tr) != 1 {
		t.Fatal("want one cabin")
	}
	tr.Observe(t0.Add(time.Second), accts("a", "b"), needs(map[string]int{"b": 6}, 8, 4))
	if cabinCount(t, tr) != 2 {
		t.Error("added account not applied at once")
	}
	tr.Observe(t0.Add(2*time.Second), accts("b"), needs(map[string]int{"b": 6}, 8, 4))
	if cabinCount(t, tr) != 1 {
		t.Error("removed account not applied at once")
	}
}

func TestTrackerCabinNeedsHoldPerAccount(t *testing.T) {
	tr := NewTracker()
	tr.Observe(t0, accts("a", "b"), needs(map[string]int{"a": 2, "b": 2}, 8, 4))
	tr.Observe(t0.Add(time.Second), accts("a", "b"), needs(map[string]int{"a": 8, "b": 2}, 8, 4))
	tr.Observe(t0.Add(40*time.Second), accts("a", "b"), needs(map[string]int{"a": 8, "b": 2}, 8, 4))
	l, _, _ := tr.Layout("rows")
	for _, p := range l.Pavilions {
		if p.Type == "cabin" && p.Account == "a" && Desks(p) != 8 {
			t.Errorf("cabin a holds %d desks, want 8", Desks(p))
		}
		if p.Type == "cabin" && p.Account == "b" && Desks(p) != 2 {
			t.Errorf("cabin b holds %d desks, want 2", Desks(p))
		}
	}
}

func TestTrackerRev(t *testing.T) {
	tr := NewTracker()
	tr.Observe(t0, accts("a"), needs(nil, 8, 4))
	_, rev1, err := tr.Layout("rows")
	if err != nil || len(rev1) < 6 {
		t.Fatalf("rev %q err %v", rev1, err)
	}
	tr.Observe(t0.Add(time.Second), accts("a"), needs(nil, 8, 4))
	_, rev2, _ := tr.Layout("rows")
	if rev1 != rev2 {
		t.Error("rev changed with nothing new")
	}
	_, other, _ := tr.Layout("village")
	if other == rev1 {
		t.Error("two templates share a rev")
	}
	tr.Observe(t0.Add(2*time.Second), accts("a", "b"), needs(nil, 8, 4))
	_, rev3, _ := tr.Layout("rows")
	if rev3 == rev1 {
		t.Error("rev did not change with a new account")
	}
	if _, _, err := tr.Layout("hall"); err == nil {
		t.Error("unknown template accepted")
	}
}

func TestTrackerLayoutBeforeObserve(t *testing.T) {
	l, _, err := NewTracker().Layout("rows")
	if err != nil || len(l.Pavilions) == 0 {
		t.Errorf("empty tracker should serve the zero account layout: %v", err)
	}
}

// a slow generator must not block Observe or other readers.
func TestObserveIsNotBlockedByGeneration(t *testing.T) {
	tr := NewTracker()
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	tr.gen = func(template string, in Input) (layout.Layout, error) {
		once.Do(func() {
			close(started)
			<-release
		})
		return Generate(template, in)
	}
	tr.Observe(time.Now(), []Account{{"a", "A"}}, Needs{})
	done := make(chan error, 1)
	go func() {
		_, _, err := tr.Layout("rows")
		done <- err
	}()
	<-started
	obs := make(chan struct{})
	go func() {
		tr.Observe(time.Now(), []Account{{"a", "A"}, {"b", "B"}}, Needs{})
		close(obs)
	}()
	select {
	case <-obs:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("Observe blocked while a layout was generated")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// the stale result must not be cached for the new version
	l, _, err := tr.Layout("rows")
	if err != nil || len(l.Pavilions) == 0 {
		t.Fatal(err)
	}
	n := 0
	for _, p := range l.Pavilions {
		if p.Type == "cabin" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("cabins = %d, want 2 (stale cache)", n)
	}
}
