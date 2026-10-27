package office

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/office/summary"
)

// a service over one invented transcript line on 2026-10-27 (utc).
func summaryHandler(t *testing.T, with bool) http.Handler {
	t.Helper()
	cfg := Config{
		Token: testToken, Port: 4242, Hosts: []string{"127.0.0.1:4242"},
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
	}
	if with {
		root := t.TempDir()
		dir := filepath.Join(root, "-p")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		line := `{"type":"assistant","timestamp":"2026-10-27T10:00:00Z","sessionId":"s1","cwd":"/p","message":{"id":"m","model":"claude-sonnet-4-5","content":[],"usage":{"input_tokens":7,"output_tokens":3}}}` + "\n"
		p := filepath.Join(dir, "s1.jsonl")
		if err := os.WriteFile(p, []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		mt := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
		_ = os.Chtimes(p, mt, mt)
		cfg.Summary = summary.New(summary.Config{
			Roots: []string{root}, Loc: time.UTC,
			Now: func() time.Time { return time.Date(2026, 10, 27, 15, 0, 0, 0, time.UTC) },
		})
	}
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSummaryRoutesGuards(t *testing.T) {
	h := summaryHandler(t, true)
	for _, target := range []string{"/api/summary?day=2026-10-27", "/api/history?days=5"} {
		if rec := get(t, h, "GET", target, nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s no token = %d, want 401", target, rec.Code)
		}
		wrongHost := func(r *http.Request) { r.Host = "evil.example:4242"; r.Header.Set("X-Clalt-Token", testToken) }
		if rec := get(t, h, "GET", target, wrongHost); rec.Code != http.StatusForbidden {
			t.Errorf("%s wrong host = %d, want 403", target, rec.Code)
		}
		if rec := get(t, h, "POST", target, withHeader(testToken)); rec.Code == http.StatusOK {
			t.Errorf("%s POST = %d, a read route", target, rec.Code)
		}
	}
}

func TestSummaryRouteReply(t *testing.T) {
	h := summaryHandler(t, true)
	rec := get(t, h, "GET", "/api/summary?day=2026-10-27", withHeader(testToken))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	var got summary.Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.V != 1 || got.Day != "2026-10-27" || got.Totals.Tokens != 10 {
		t.Errorf("%+v", got)
	}
	// no day means today
	rec = get(t, h, "GET", "/api/summary", withHeader(testToken))
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != http.StatusOK || got.Day != "2026-10-27" {
		t.Errorf("default day: %d %s", rec.Code, got.Day)
	}
}

func TestHistoryRouteReply(t *testing.T) {
	h := summaryHandler(t, true)
	rec := get(t, h, "GET", "/api/history?days=2", withHeader(testToken))
	var got summary.History
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("%d %v", rec.Code, err)
	}
	if got.Days != 2 || got.To != "2026-10-27" || len(got.Accounts) != 1 || got.Accounts[0].Days[1].Tokens != 10 {
		t.Errorf("%+v", got)
	}
	rec = get(t, h, "GET", "/api/history", withHeader(testToken))
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Days != 30 {
		t.Errorf("default days %d", got.Days)
	}
}

func TestSummaryRoutesBadParams(t *testing.T) {
	h := summaryHandler(t, true)
	for _, target := range []string{
		"/api/summary?day=yesterday", "/api/summary?day=2026-10-27&day=2026-10-26", "/api/summary?day=2026-02-30",
		"/api/history?days=0", "/api/history?days=91", "/api/history?days=abc", "/api/history?days=-3", "/api/history?days=5&days=6", "/api/history?days=",
	} {
		if rec := get(t, h, "GET", target, withHeader(testToken)); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", target, rec.Code)
		}
	}
}

func TestSummaryRoutesWithoutAServiceAre404(t *testing.T) {
	h := summaryHandler(t, false)
	for _, target := range []string{"/api/summary", "/api/history"} {
		if rec := get(t, h, "GET", target, withHeader(testToken)); rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", target, rec.Code)
		}
	}
}
