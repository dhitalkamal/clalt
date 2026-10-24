package garden

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"sync"
	"time"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// holds before a desk need changes the campus. a short subagent spike never
// rebuilds it, it uses overflow spots instead.
const (
	GrowHold   = 30 * time.Second
	ShrinkHold = 10 * time.Minute
)

// Needs is what the sessions ask for right now.
type Needs struct {
	CabinDesks     map[string]int
	WorkspaceDesks int
	ResearchDesks  int
}

// held is one desk count with its pending change.
type held struct {
	val   int
	dir   int // +1 growing, -1 shrinking, 0 steady
	since time.Time
	cand  int // lowest need seen while growing, highest while shrinking
}

// observe folds in one reading and reports whether the applied value moved.
func (h *held) observe(now time.Time, v int) bool {
	switch {
	case v == h.val:
		h.dir = 0
		return false
	case v > h.val:
		if h.dir != 1 {
			h.dir, h.since, h.cand = 1, now, v
		}
		h.cand = min(h.cand, v)
		if now.Sub(h.since) >= GrowHold {
			h.val, h.dir = h.cand, 0
			return true
		}
	default:
		if h.dir != -1 {
			h.dir, h.since, h.cand = -1, now, v
		}
		h.cand = max(h.cand, v)
		if now.Sub(h.since) >= ShrinkHold {
			h.val, h.dir = h.cand, 0
			return true
		}
	}
	return false
}

type cached struct {
	version int
	layout  layout.Layout
	rev     string
}

// Tracker holds the desk needs with their grow and shrink holds and serves
// the generated layouts. it is safe for concurrent use.
type Tracker struct {
	mu       sync.Mutex
	accounts []Account
	cabin    map[string]*held
	ws, rs   *held
	version  int
	cache    map[string]cached
	gen      func(string, Input) (layout.Layout, error) // Generate, swapped in tests
}

func NewTracker() *Tracker {
	return &Tracker{cabin: map[string]*held{}, cache: map[string]cached{}, gen: Generate}
}

// Observe feeds one reading. a changed account list applies at once; desk
// needs follow the holds.
func (t *Tracker) Observe(now time.Time, accounts []Account, needs Needs) {
	t.mu.Lock()
	defer t.mu.Unlock()
	changed := !slices.Equal(t.accounts, accounts)
	t.accounts = slices.Clone(accounts)
	keep := map[string]bool{}
	for _, a := range accounts {
		keep[a.ID] = true
		v := max(MinCabinDesks, min(needs.CabinDesks[a.ID], maxCabinDesks))
		if h, ok := t.cabin[a.ID]; ok {
			changed = h.observe(now, v) || changed
		} else {
			t.cabin[a.ID], changed = &held{val: v}, true
		}
	}
	for id := range t.cabin {
		if !keep[id] {
			delete(t.cabin, id)
			changed = true
		}
	}
	changed = observePool(&t.ws, now, max(MinWorkspaceDesks, min(needs.WorkspaceDesks, maxPoolDesks))) || changed
	changed = observePool(&t.rs, now, max(MinResearchDesks, min(needs.ResearchDesks, maxPoolDesks))) || changed
	if changed {
		t.version++
	}
}

// observePool feeds a shared desk count, creating it on the first reading.
func observePool(h **held, now time.Time, v int) bool {
	if *h == nil {
		*h = &held{val: v}
		return true
	}
	return (*h).observe(now, v)
}

// input is the generator input for what is applied now.
func (t *Tracker) input() Input {
	in := Input{Accounts: slices.Clone(t.accounts), CabinDesks: map[string]int{}, WorkspaceDesks: MinWorkspaceDesks, ResearchDesks: MinResearchDesks}
	for id, h := range t.cabin {
		in.CabinDesks[id] = h.val
	}
	if t.ws != nil {
		in.WorkspaceDesks = t.ws.val
	}
	if t.rs != nil {
		in.ResearchDesks = t.rs.val
	}
	return in
}

// Layout is the layout of a template for the applied needs and a short rev
// that changes exactly when the layout does. do not modify the result.
func (t *Tracker) Layout(template string) (layout.Layout, string, error) {
	t.mu.Lock()
	if c, ok := t.cache[template]; ok && c.version == t.version {
		t.mu.Unlock()
		return c.layout, c.rev, nil
	}
	// generate on a copy outside the lock: big layouts take seconds and
	// Observe must not wait for them
	in, version := t.input(), t.version
	t.mu.Unlock()
	l, err := t.gen(template, in)
	if err != nil {
		return layout.Layout{}, "", err
	}
	b, err := layout.Encode(l)
	if err != nil {
		return layout.Layout{}, "", err
	}
	sum := sha256.Sum256(b)
	rev := hex.EncodeToString(sum[:])[:12]
	t.mu.Lock()
	// keep the result only when the needs did not move meanwhile
	if t.version == version {
		t.cache[template] = cached{version, l, rev}
	}
	t.mu.Unlock()
	return l, rev, nil
}
