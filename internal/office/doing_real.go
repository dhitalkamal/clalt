package office

import (
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dhitalkamal/clalt/internal/office/activity"
)

// doingTail is how much of a transcript's end is read, and doingEntries how
// many entries of it are looked at.
const (
	doingTail    = 256 << 10
	doingEntries = 40
	doingMaxKeep = 200 // cached files before the cache is dropped
)

// doingReads counts transcript reads, for the cache test.
var doingReads atomic.Int64

type doingKey struct {
	size  int64
	mtime time.Time
}

// doingCached is the newest tool call of a file as of a size and mtime. the
// freshness check is made on each call, not stored.
type doingCached struct {
	key doingKey
	d   *Doing
}

var (
	doingMu    sync.Mutex
	doingCache = map[string]doingCached{}
)

// Doing is the work of a session (agent empty) or of one of its subagents, nil
// when there is none that is fresh. it reads the transcript tail only when the
// file changed since the last call.
func (r Real) Doing(handle, agent string) *Doing {
	src, err := r.Transcript(handle, agent)
	if err != nil {
		return nil
	}
	fi, err := os.Stat(src.Path)
	if err != nil {
		return nil
	}
	now := r.Now
	if now == nil {
		now = time.Now
	}
	k := doingKey{fi.Size(), fi.ModTime()}
	// a legacy subagent shares the main file, so the agent is part of the key
	id := src.Path + "\x00" + src.Agent
	doingMu.Lock()
	c, ok := doingCache[id]
	doingMu.Unlock()
	if !ok || c.key != k {
		c = doingCached{key: k, d: readDoing(src)}
		doingMu.Lock()
		if len(doingCache) >= doingMaxKeep {
			doingCache = map[string]doingCached{}
		}
		doingCache[id] = c
		doingMu.Unlock()
	}
	if c.d == nil || now().Sub(c.d.At) > doingFresh {
		return nil
	}
	d := *c.d
	return &d
}

// readDoing reads the tail and finds the newest call, stale or not.
func readDoing(src activity.Source) *Doing {
	doingReads.Add(1)
	rep, err := activity.Read(activity.Options{Source: src, Limit: doingEntries, MaxScan: doingTail})
	if err != nil {
		return nil
	}
	// staleness is judged by the caller, so judge it at the call's own time
	var at time.Time
	for _, e := range rep.Entries {
		if e.Kind == activity.KindTool {
			at, _ = time.Parse(time.RFC3339, e.T)
			break
		}
	}
	return doingOf(rep.Entries, at)
}
