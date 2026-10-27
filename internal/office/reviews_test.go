package office

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/reviews"
)

type fakeReviews struct {
	list    []reviews.Review
	status  reviews.Status
	saved   *reviews.Config
	saveErr error
}

func (f *fakeReviews) Status() reviews.Status          { return f.status }
func (f *fakeReviews) List() ([]reviews.Review, error) { return f.list, nil }
func (f *fakeReviews) Get(id string) (reviews.Review, bool, error) {
	for _, r := range f.list {
		if r.ID() == id {
			return r, true, nil
		}
	}
	return reviews.Review{}, false, nil
}
func (f *fakeReviews) SettingsView(context.Context) reviews.SettingsView {
	return reviews.SettingsView{Enabled: f.status.Enabled, Repos: []string{"o/r"}, Suggestions: []string{"o/z"}}
}
func (f *fakeReviews) SaveSettings(c reviews.Config) error {
	f.saved = &c
	return f.saveErr
}

func reviewEnv(t *testing.T, f *fakeReviews) layoutEnv {
	t.Helper()
	e := newLayoutEnv(t)
	h, err := NewHandler(Config{
		Token: testToken, Port: 4242, Hosts: []string{"127.0.0.1:4242"}, Layouts: e.store, Reviews: f,
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	e.h = h
	return e
}

func oneReview() reviews.Review {
	return reviews.Review{Repo: "o/r", Number: 7, Title: "Fix", Author: "ann", URL: "u", SHA: "1111111aaaa",
		StartedAt: time.Unix(1, 0).UTC(), EndedAt: time.Unix(2, 0).UTC(), Account: "home", Transcript: "sess-9",
		Findings: []reviews.Finding{{Severity: "high", File: "a.go", Line: 3, Title: "t", Detail: "d"}, {Severity: "low", Title: "n"}}, Summary: "s"}
}

func getAuthed(e layoutEnv, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", target, nil)
	req.Host = "127.0.0.1:4242"
	asCookie(req)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func TestReviewsListHasStatusAndSummaries(t *testing.T) {
	f := &fakeReviews{list: []reviews.Review{oneReview()}, status: reviews.Status{Enabled: true, Count: 1}}
	rec := getAuthed(reviewEnv(t, f), "/api/reviews")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var got struct {
		Enabled bool `json:"enabled"`
		Reviews []struct {
			ID     string         `json:"id"`
			Repo   string         `json:"repo"`
			Counts map[string]int `json:"counts"`
			Handle string         `json:"handle"`
			Find   any            `json:"findings"`
		} `json:"reviews"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Enabled || len(got.Reviews) != 1 || got.Reviews[0].ID != "o-r-7-1111111" || got.Reviews[0].Counts["high"] != 1 || got.Reviews[0].Counts["low"] != 1 || got.Reviews[0].Handle != "ext-sess-9" {
		t.Fatalf("%s", rec.Body)
	}
	if got.Reviews[0].Find != nil {
		t.Fatal("the list carries no findings")
	}
}

func TestReviewOneAndNotFound(t *testing.T) {
	e := reviewEnv(t, &fakeReviews{list: []reviews.Review{oneReview()}})
	rec := getAuthed(e, "/api/reviews/o-r-7-1111111")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"detail":"d"`) || !strings.Contains(rec.Body.String(), `"handle":"ext-sess-9"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for _, p := range []string{"/api/reviews/o-r-9-1111111", "/api/reviews/..%2Fx", "/api/reviews/a/b"} {
		if c := getAuthed(e, p).Code; c != 404 {
			t.Errorf("%s: %d", p, c)
		}
	}
}

func TestReviewsNeedTheToken(t *testing.T) {
	e := reviewEnv(t, &fakeReviews{})
	req := httptest.NewRequest("GET", "/api/reviews", nil)
	req.Host = "127.0.0.1:4242"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("%d", rec.Code)
	}
}

func TestReviewsWithoutServiceAre404(t *testing.T) {
	e := newLayoutEnv(t)
	if c := getAuthed(e, "/api/reviews").Code; c != 404 {
		t.Fatalf("%d", c)
	}
}

func TestReviewSettingsGetAndSave(t *testing.T) {
	f := &fakeReviews{}
	e := reviewEnv(t, f)
	if rec := getAuthed(e, "/api/reviews/settings"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"suggestions":["o/z"]`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec := e.send("POST", "/api/reviews/settings", `{"enabled":true,"repos":["o/r","o/z"]}`)
	if rec.Code != 200 || f.saved == nil || !f.saved.Enabled || len(f.saved.Repos) != 2 {
		t.Fatalf("%d %s %+v", rec.Code, rec.Body, f.saved)
	}
}

func TestReviewSettingsSaveGuards(t *testing.T) {
	f := &fakeReviews{}
	e := reviewEnv(t, f)
	body := `{"enabled":true,"repos":[]}`
	cases := map[string]*httptest.ResponseRecorder{
		"no edit header": e.send("POST", "/api/reviews/settings", body, drop("X-Clalt-Edit")),
		"foreign origin": e.send("POST", "/api/reviews/settings", body, set("Origin", "http://evil.example")),
		"no cookie":      e.send("POST", "/api/reviews/settings", body, noCookie),
		"wrong type":     e.send("POST", "/api/reviews/settings", body, set("Content-Type", "text/plain")),
		"put":            e.send("PUT", "/api/reviews/settings", body),
	}
	want := map[string]int{"no edit header": 403, "foreign origin": 403, "no cookie": 401, "wrong type": 415, "put": 405}
	for k, rec := range cases {
		if rec.Code != want[k] {
			t.Errorf("%s: %d", k, rec.Code)
		}
	}
	for name, b := range map[string]string{"bad repo": `{"enabled":true,"repos":["--web"]}`, "junk": `nope`, "unknown field": `{"enabled":true,"repos":[],"x":1}`} {
		if rec := e.send("POST", "/api/reviews/settings", b); rec.Code != 400 {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	if f.saved != nil {
		t.Fatalf("a refused request saved %+v", f.saved)
	}
}

func TestSnapshotMarksReviewerDeskBusyWhileReviewing(t *testing.T) {
	src := Sources{Accounts: staticAccounts{}, Sessions: fakeSessions{}, Limits: noLimits{}}
	src.Reviews = &fakeReviews{status: reviews.Status{Enabled: true, Running: &reviews.Running{Repo: "o/r", Number: 7}, Count: 2}}
	snap, err := Build(src)
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Reviews.Enabled || snap.Reviews.Count != 2 || snap.Reviews.Running == nil {
		t.Fatalf("reviews = %+v", snap.Reviews)
	}
	var desk *TeamOut
	for i := range snap.Team {
		if snap.Team[i].ID == "pr-reviewer" {
			desk = &snap.Team[i]
		}
	}
	if desk == nil || desk.Busy != 1 {
		t.Fatalf("team = %+v", snap.Team)
	}
	src.Reviews = &fakeReviews{status: reviews.Status{Enabled: true}}
	snap, _ = Build(src)
	for _, m := range snap.Team {
		if m.ID == "pr-reviewer" && m.Busy != 0 {
			t.Fatal("idle desk is busy")
		}
	}
	src.Reviews = nil
	snap, _ = Build(src)
	for _, m := range snap.Team {
		if m.ID == "pr-reviewer" {
			t.Fatal("no reviews, no desk")
		}
	}
}

var _ = http.StatusOK
