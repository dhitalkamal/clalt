package office

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/dhitalkamal/clalt/internal/office/summary"
)

// summaryRoute serves GET /api/summary?day=YYYY-MM-DD, see
// docs/plans/office-work.md. no day is today.
func (h *handler) summaryRoute(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	day, ok := one(q, "day")
	if !ok || (q.Has("day") && day == "") {
		http.Error(w, "day must be given once as YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	if h.cfg.Summary == nil {
		http.Error(w, "summary is not available", http.StatusNotFound)
		return
	}
	sum, err := h.cfg.Summary.Day(day)
	if errors.Is(err, summary.ErrBadDay) {
		http.Error(w, "day must be YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	h.writeSummary(w, sum, err, "summary")
}

// historyRoute serves GET /api/history?days=N, 1 to 90, 30 when not given.
func (h *handler) historyRoute(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	raw, ok := one(q, "days")
	days := 0
	if q.Has("days") {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > summary.MaxDays {
			ok = false
		}
		days = n
	}
	if !ok {
		http.Error(w, "days must be given once, from 1 to 90", http.StatusBadRequest)
		return
	}
	if h.cfg.Summary == nil {
		http.Error(w, "history is not available", http.StatusNotFound)
		return
	}
	hist, err := h.cfg.Summary.History(days)
	if errors.Is(err, summary.ErrBadDays) {
		http.Error(w, "days must be from 1 to 90", http.StatusBadRequest)
		return
	}
	h.writeSummary(w, hist, err, "history")
}

// writeSummary encodes a reply, or says it failed without saying why.
func (h *handler) writeSummary(w http.ResponseWriter, v any, err error, what string) {
	if err != nil {
		h.logf("%s failed: %v", what, err)
		http.Error(w, what+" failed", http.StatusInternalServerError)
		return
	}
	body, err := json.Marshal(v)
	if err != nil {
		h.logf("encoding the %s failed: %v", what, err)
		http.Error(w, what+" failed", http.StatusInternalServerError)
		return
	}
	writeActivity(w, body)
}
