package office

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/office/activity"
)

// ActivitySource maps a session handle (and a subagent id, empty for the main
// conversation) to the transcript to read. a session that has none is a
// *NotFoundError.
type ActivitySource interface {
	Transcript(handle, agent string) (activity.Source, error)
}

// NotFoundError says why no transcript was found. the reason is shown to the
// page, so it names no path.
type NotFoundError struct{ Reason string }

func (e *NotFoundError) Error() string { return e.Reason }

// activityTTL is how long a reply is reused, so a page that polls and a second
// window do not read the same file twice a second.
const activityTTL = time.Second

// maxActivityCache bounds the reply cache. entries are up to 2 MB, so it is
// small, and it is emptied when full.
const maxActivityCache = 32

var agentID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

type cachedReply struct {
	at   time.Time
	body []byte
}

// activityCache holds the replies of the last second by request.
type activityCache struct {
	mu sync.Mutex
	m  map[string]cachedReply
}

func (c *activityCache) get(key string, now time.Time) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.m[key]
	if !ok || now.Sub(r.at) >= activityTTL || now.Before(r.at) {
		return nil, false
	}
	return r.body, true
}

func (c *activityCache) put(key string, body []byte, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]cachedReply{}
	}
	for k, r := range c.m {
		if now.Sub(r.at) >= activityTTL {
			delete(c.m, k)
		}
	}
	if len(c.m) >= maxActivityCache {
		clear(c.m)
	}
	c.m[key] = cachedReply{at: now, body: body}
}

// one reads a parameter that must be given at most once.
func one(q map[string][]string, name string) (string, bool) {
	v := q[name]
	if len(v) > 1 {
		return "", false
	}
	if len(v) == 0 {
		return "", true
	}
	return v[0], true
}

// activity serves GET /api/activity, see docs/plans/office-work.md.
func (h *handler) activity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	handle, ok1 := one(q, "handle")
	agent, ok2 := one(q, "agent")
	before, ok3 := one(q, "before")
	after, ok5 := one(q, "after")
	limitS, ok4 := one(q, "limit")
	limit := 0
	if limitS != "" {
		n, err := strconv.Atoi(limitS)
		if err != nil || n < 1 {
			http.Error(w, "limit must be a positive number", http.StatusBadRequest)
			return
		}
		limit = n
	}
	switch {
	case !ok1 || !ok2 || !ok3 || !ok4 || !ok5:
		http.Error(w, "a parameter was given twice", http.StatusBadRequest)
		return
	case !feed.ValidKey(handle):
		http.Error(w, "handle is not a session handle", http.StatusBadRequest)
		return
	case agent != "" && !agentID.MatchString(agent):
		http.Error(w, "agent is not a subagent id", http.StatusBadRequest)
		return
	case len(before) > 100 || len(after) > 100:
		http.Error(w, "before and after are cursors", http.StatusBadRequest)
		return
	}
	if h.cfg.Activity == nil {
		http.Error(w, "activity is not available", http.StatusNotFound)
		return
	}
	now := h.now()
	key := handle + "\x00" + agent + "\x00" + before + "\x00" + after + "\x00" + strconv.Itoa(limit)
	if body, ok := h.actCache.get(key, now); ok {
		writeActivity(w, body)
		return
	}
	src, err := h.cfg.Activity.Transcript(handle, agent)
	var nf *NotFoundError
	if errors.As(err, &nf) {
		http.Error(w, "not found: "+nf.Reason, http.StatusNotFound)
		return
	}
	if err != nil {
		h.logf("activity of %s failed: %v", handle, err)
		http.Error(w, "activity failed", http.StatusInternalServerError)
		return
	}
	rep, err := activity.Read(activity.Options{Source: src, Before: before, After: after, Limit: limit})
	switch {
	case errors.Is(err, activity.ErrBadCursor):
		http.Error(w, "a cursor is not one of this log", http.StatusBadRequest)
		return
	case errors.Is(err, fs.ErrNotExist):
		http.Error(w, "not found: the transcript is gone", http.StatusNotFound)
		return
	case err != nil:
		h.logf("reading the activity of %s failed: %v", handle, err)
		http.Error(w, "activity failed", http.StatusInternalServerError)
		return
	}
	rep.Handle = handle
	body, err := json.Marshal(rep)
	if err != nil {
		h.logf("encoding the activity of %s failed: %v", handle, err)
		http.Error(w, "activity failed", http.StatusInternalServerError)
		return
	}
	h.actCache.put(key, body, now)
	writeActivity(w, body)
}

func writeActivity(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
}
