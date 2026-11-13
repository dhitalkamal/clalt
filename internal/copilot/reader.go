package copilot

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
)

// limits that keep a huge or hostile folder from stalling the office.
const (
	defaultFresh     = 6 * time.Hour
	defaultMaxDirs   = 40
	defaultTailBytes = 256 << 10
)

// Session is one open Copilot session. no free text lives here.
type Session struct {
	ID     string
	Handle string
	Cwd    string
	Model  string
	State  feed.State
	// Reason is why the session needs you, set only while State is needs-you.
	Reason    string
	StartedAt time.Time
	// UpdatedAt is when events.jsonl last changed.
	UpdatedAt time.Time
}

// Reader lists the open sessions in a Copilot home. zero limits mean the
// defaults, Now is time.Now when nil.
type Reader struct {
	Home      string
	Now       func() time.Time
	Fresh     time.Duration
	MaxDirs   int
	TailBytes int64
	// FirstReadBytes is the largest file folded whole on its first read.
	FirstReadBytes int64

	mu    sync.Mutex
	cache map[string]*entry
}

// Sessions lists the sessions whose events file changed within the freshness
// window and that did not shut down, newest first. it only reads.
func (r *Reader) Sessions() ([]Session, error) {
	if r.Home == "" {
		return nil, nil
	}
	now, fresh, maxDirs, tail := time.Now, r.Fresh, r.MaxDirs, r.TailBytes
	if r.Now != nil {
		now = r.Now
	}
	if fresh <= 0 {
		fresh = defaultFresh
	}
	if maxDirs <= 0 {
		maxDirs = defaultMaxDirs
	}
	if tail <= 0 {
		tail = defaultTailBytes
	}
	root := filepath.Join(r.Home, "session-state")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	type cand struct {
		id  string
		mod time.Time
	}
	var cands []cand
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, ok := feed.CopilotHandle(e.Name()); !ok {
			continue
		}
		fi, err := os.Lstat(filepath.Join(root, e.Name(), "events.jsonl"))
		if err != nil || !fi.Mode().IsRegular() || now().Sub(fi.ModTime()) > fresh {
			continue
		}
		cands = append(cands, cand{e.Name(), fi.ModTime()})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].mod.After(cands[j].mod) })
	if len(cands) > maxDirs {
		cands = cands[:maxDirs]
	}
	var out []Session
	r.mu.Lock()
	defer r.mu.Unlock()
	live := map[string]bool{}
	for _, c := range cands {
		live[c.id] = true
		dir := filepath.Join(root, c.id)
		sum, err := r.read(c.id, filepath.Join(dir, "events.jsonl"), tail)
		if err != nil || sum.Ended {
			continue
		}
		ws := readWorkspace(filepath.Join(dir, "workspace.yaml"))
		s := Session{ID: c.id, State: sum.State, Reason: sum.Reason, Model: sum.Model, Cwd: ws.Cwd, StartedAt: sum.StartedAt, UpdatedAt: c.mod}
		s.Handle, _ = feed.CopilotHandle(c.id)
		if s.Cwd == "" {
			s.Cwd = sum.Cwd
		}
		if s.StartedAt.IsZero() {
			s.StartedAt = ws.CreatedAt
		}
		out = append(out, s)
	}
	for id := range r.cache {
		if !live[id] {
			delete(r.cache, id)
		}
	}
	return out, nil
}
