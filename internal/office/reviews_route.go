package office

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/reviews"
)

// ReviewSource is the PR review service as the office sees it.
type ReviewSource interface {
	Status() reviews.Status
	List() ([]reviews.Review, error)
	Get(id string) (reviews.Review, bool, error)
	SettingsView(ctx context.Context) reviews.SettingsView
	SaveSettings(c reviews.Config) error
}

// ReviewsOut is the snapshot's view of the reviews: whether they are on, what
// runs now and how many there are.
type ReviewsOut struct {
	Enabled bool             `json:"enabled"`
	Running *reviews.Running `json:"running"`
	Count   int              `json:"count"`
}

// reviewDesk is the team member that stands for the reviewer desk. its id
// matches the page's review rule, so it sits in the review space.
const reviewDesk = "pr-reviewer"

// withReviews adds the status to the snapshot and, when reviews are on, the
// reviewer desk right after the built-in team, busy while a review runs.
func withReviews(src ReviewSource, team []TeamOut) (ReviewsOut, []TeamOut) {
	if src == nil {
		return ReviewsOut{}, team
	}
	st := src.Status()
	out := ReviewsOut{Enabled: st.Enabled, Running: st.Running, Count: st.Count}
	if !st.Enabled {
		return out, team
	}
	m := TeamOut{ID: reviewDesk, Name: "PR reviewer", Desc: "reviews the pull requests that ask for you", Source: "reviews", Color: teamColor(reviewDesk)}
	if st.Running != nil {
		m.Busy = 1
	}
	at := len(builtinTeam)
	if at > len(team) {
		at = len(team)
	}
	team = append(team[:at:at], append([]TeamOut{m}, team[at:]...)...)
	if len(team) > maxTeam {
		team = team[:maxTeam]
	}
	return out, team
}

// reviewHandle is the activity handle that opens the run log of a review: the
// claude session id as a plain claude session, which the activity reader finds
// by scanning the projects folder. empty when the id is not usable as one.
func reviewHandle(sessionID string) string {
	h := "ext-" + sessionID
	if sessionID == "" || !feed.IsExt(h) {
		return ""
	}
	return h
}

type reviewRow struct {
	ID         string         `json:"id"`
	Repo       string         `json:"repo"`
	Number     int            `json:"number"`
	Title      string         `json:"title"`
	Author     string         `json:"author"`
	URL        string         `json:"url"`
	SHA        string         `json:"sha"`
	StartedAt  time.Time      `json:"startedAt"`
	EndedAt    time.Time      `json:"endedAt"`
	Account    string         `json:"account"`
	Counts     map[string]int `json:"counts"`
	Skipped    string         `json:"skipped,omitempty"`
	Error      string         `json:"error,omitempty"`
	Handle     string         `json:"handle"`
	Transcript string         `json:"transcript"`
	// state is open, merged, closed or unknown, never empty. stateCheckedAt is
	// null until the state was looked up.
	State          string     `json:"state"`
	StateCheckedAt *time.Time `json:"stateCheckedAt"`
	// retry info for failed reviews, both omitted when unset
	Attempts    int        `json:"attempts,omitempty"`
	NextRetryAt *time.Time `json:"nextRetryAt,omitempty"`
}

func rowOf(r reviews.Review) reviewRow {
	counts := map[string]int{"high": 0, "medium": 0, "low": 0}
	for _, f := range r.Findings {
		counts[f.Severity]++
	}
	state := reviews.StateUnknown
	switch r.State {
	case reviews.StateOpen, reviews.StateMerged, reviews.StateClosed:
		state = r.State
	}
	return reviewRow{ID: r.ID(), Repo: r.Repo, Number: r.Number, Title: r.Title, Author: r.Author, URL: r.URL, SHA: r.SHA,
		StartedAt: r.StartedAt, EndedAt: r.EndedAt, Account: r.Account, Counts: counts, Skipped: r.Skipped, Error: r.Error,
		Handle: reviewHandle(r.Transcript), Transcript: r.Transcript,
		State: state, StateCheckedAt: r.StateCheckedAt,
		Attempts: r.Attempts, NextRetryAt: r.NextRetryAt}
}

type reviewFull struct {
	reviewRow
	Findings []reviews.Finding `json:"findings"`
	Summary  string            `json:"summary"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	w.WriteHeader(code)
	_, _ = w.Write(raw)
}

// reviewsGet serves the read routes: /api/reviews, /api/reviews/settings and
// /api/reviews/<id>. the writes are in reviewsSave.
func (h *handler) reviewsGet(w http.ResponseWriter, r *http.Request) {
	if h.cfg.Reviews == nil {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/api/reviews"), "/")
	switch rest {
	case "":
		h.reviewsList(w)
	case "settings":
		writeJSON(w, 200, h.cfg.Reviews.SettingsView(r.Context()))
	default:
		rv, ok, err := h.cfg.Reviews.Get(rest)
		if err != nil {
			h.logf("reading review %s failed: %v", rest, err)
			http.Error(w, "review failed", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, reviewFull{reviewRow: rowOf(rv), Findings: rv.Findings, Summary: rv.Summary})
	}
}

func (h *handler) reviewsList(w http.ResponseWriter) {
	list, err := h.cfg.Reviews.List()
	if err != nil {
		h.logf("listing reviews failed: %v", err)
		http.Error(w, "reviews failed", http.StatusInternalServerError)
		return
	}
	rows := make([]reviewRow, len(list))
	for i, rv := range list {
		rows[i] = rowOf(rv)
	}
	st := h.cfg.Reviews.Status()
	writeJSON(w, 200, map[string]any{"enabled": st.Enabled, "running": st.Running, "lastChecked": st.LastChecked, "maxAttempts": reviews.MaxAttempts(), "reviews": rows})
}

// reviewsSave is POST /api/reviews/settings, after the write guards. the body
// is {"enabled": bool, "repos": ["owner/name"]} and nothing else.
func (h *handler) reviewsSave(w http.ResponseWriter, body []byte) {
	if h.cfg.Reviews == nil {
		http.Error(w, "reviews are not available", http.StatusNotFound)
		return
	}
	var in reviews.Config
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || dec.More() {
		http.Error(w, "body must be {\"enabled\": bool, \"repos\": [\"owner/name\"]}", http.StatusBadRequest)
		return
	}
	if _, err := dec.Token(); err != io.EOF {
		http.Error(w, "body must be one json object", http.StatusBadRequest)
		return
	}
	for _, repo := range in.Repos {
		if !reviews.ValidRepo(repo) {
			http.Error(w, "not a repository (owner/name): "+clean(repo, 60), http.StatusBadRequest)
			return
		}
	}
	if err := h.cfg.Reviews.SaveSettings(in); err != nil {
		h.logf("saving review settings failed: %v", err)
		http.Error(w, "could not save the settings", http.StatusInternalServerError)
		return
	}
	writeJSON(w, 200, h.cfg.Reviews.SettingsView(context.Background()))
}
