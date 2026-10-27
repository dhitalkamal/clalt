package office

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/office/activity"
)

// fakeActivity answers for one handle and counts how often it is asked.
type fakeActivity struct {
	calls int
	src   activity.Source
}

func (f *fakeActivity) Transcript(handle, agent string) (activity.Source, error) {
	f.calls++
	if handle != "0123456789abcdef" {
		return activity.Source{}, &NotFoundError{Reason: "no such session"}
	}
	s := f.src
	s.Agent = agent
	return s, nil
}

func activityHandler(t *testing.T, src ActivitySource) http.Handler {
	t.Helper()
	h, err := NewHandler(Config{
		Token: testToken, Port: 4242, Hosts: []string{"127.0.0.1:4242"}, Activity: src,
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func activityURL(params map[string]string) string {
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return "/api/activity?" + q.Encode()
}

const goodHandle = "0123456789abcdef"

func TestActivityRouteGuards(t *testing.T) {
	h := activityHandler(t, &fakeActivity{src: activity.Source{Path: "activity/testdata/main.jsonl"}})
	target := activityURL(map[string]string{"handle": goodHandle})
	if rec := get(t, h, "GET", target, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token = %d, want 401", rec.Code)
	}
	wrongHost := func(r *http.Request) { r.Host = "evil.example:4242"; r.Header.Set("X-Clalt-Token", testToken) }
	if rec := get(t, h, "GET", target, wrongHost); rec.Code != http.StatusForbidden {
		t.Errorf("wrong host = %d, want 403", rec.Code)
	}
	if rec := get(t, h, "POST", target, withHeader(testToken)); rec.Code == http.StatusOK {
		t.Errorf("POST = %d, a read route", rec.Code)
	}
}

func TestActivityRouteReply(t *testing.T) {
	h := activityHandler(t, &fakeActivity{src: activity.Source{Path: "activity/testdata/main.jsonl"}})
	rec := get(t, h, "GET", activityURL(map[string]string{"handle": goodHandle, "limit": "3"}), withHeader(testToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d body %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content type %q", ct)
	}
	var r activity.Reply
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Handle != goodHandle || len(r.Entries) < 3 || r.Next == "" {
		t.Errorf("reply = %+v", r)
	}
	// the cursor leads on to older entries
	rec = get(t, h, "GET", activityURL(map[string]string{"handle": goodHandle, "limit": "3", "before": r.Next}), withHeader(testToken))
	var older activity.Reply
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &older) != nil || len(older.Entries) == 0 || older.Entries[0].T > r.Entries[len(r.Entries)-1].T {
		t.Errorf("older page = %d %s", rec.Code, rec.Body)
	}
}

func TestActivityRouteAgentIsPassedOn(t *testing.T) {
	h := activityHandler(t, &fakeActivity{src: activity.Source{Path: "activity/testdata/agent-a111.jsonl"}})
	rec := get(t, h, "GET", activityURL(map[string]string{"handle": goodHandle, "agent": "a111"}), withHeader(testToken))
	var r activity.Reply
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &r) != nil || r.Agent != "a111" || r.Entries[0].Agent != "a111" {
		t.Errorf("reply = %d %s", rec.Code, rec.Body)
	}
}

func TestActivityRouteUnknownHandleIs404WithAReason(t *testing.T) {
	h := activityHandler(t, &fakeActivity{})
	rec := get(t, h, "GET", activityURL(map[string]string{"handle": "ffffffffffffffff"}), withHeader(testToken))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "no such session") {
		t.Errorf("code %d body %q", rec.Code, rec.Body)
	}
}

func TestActivityRouteWithoutASourceIs404(t *testing.T) {
	h := activityHandler(t, nil)
	rec := get(t, h, "GET", activityURL(map[string]string{"handle": goodHandle}), withHeader(testToken))
	if rec.Code != http.StatusNotFound {
		t.Errorf("code %d", rec.Code)
	}
}

func TestActivityRouteBadParams(t *testing.T) {
	h := activityHandler(t, &fakeActivity{src: activity.Source{Path: "activity/testdata/main.jsonl"}})
	cases := map[string]map[string]string{
		"no handle":      {},
		"bad handle":     {"handle": "../../etc/passwd"},
		"long handle":    {"handle": strings.Repeat("a", 80)},
		"bad agent":      {"handle": goodHandle, "agent": "../x"},
		"limit text":     {"handle": goodHandle, "limit": "many"},
		"limit zero":     {"handle": goodHandle, "limit": "0"},
		"limit negative": {"handle": goodHandle, "limit": "-3"},
		"bad cursor":     {"handle": goodHandle, "before": "nope"},
	}
	for name, params := range cases {
		if rec := get(t, h, "GET", activityURL(params), withHeader(testToken)); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400 (%s)", name, rec.Code, rec.Body)
		}
	}
	// a repeated parameter is as bad as a wrong one
	if rec := get(t, h, "GET", "/api/activity?handle="+goodHandle+"&handle="+goodHandle, withHeader(testToken)); rec.Code != http.StatusBadRequest {
		t.Errorf("repeated handle = %d", rec.Code)
	}
}

func TestActivityLimitAboveTheMaxIsClamped(t *testing.T) {
	h := activityHandler(t, &fakeActivity{src: activity.Source{Path: "activity/testdata/main.jsonl"}})
	rec := get(t, h, "GET", activityURL(map[string]string{"handle": goodHandle, "limit": "100000"}), withHeader(testToken))
	if rec.Code != 200 {
		t.Errorf("code %d", rec.Code)
	}
}

func TestActivityIsCachedForOneSecondPerKey(t *testing.T) {
	src := &fakeActivity{src: activity.Source{Path: "activity/testdata/main.jsonl"}}
	h := activityHandler(t, src)
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	h.(*handler).now = func() time.Time { return now }
	target := activityURL(map[string]string{"handle": goodHandle})
	first := get(t, h, "GET", target, withHeader(testToken))
	now = now.Add(900 * time.Millisecond)
	second := get(t, h, "GET", target, withHeader(testToken))
	if src.calls != 1 || first.Body.String() != second.Body.String() {
		t.Errorf("calls %d, bodies equal %v", src.calls, first.Body.String() == second.Body.String())
	}
	// another key is its own entry
	get(t, h, "GET", activityURL(map[string]string{"handle": goodHandle, "limit": "5"}), withHeader(testToken))
	if src.calls != 2 {
		t.Errorf("calls after another limit = %d", src.calls)
	}
	now = now.Add(200 * time.Millisecond)
	get(t, h, "GET", target, withHeader(testToken))
	if src.calls != 3 {
		t.Errorf("calls after the second = %d, want 3", src.calls)
	}
}

func TestActivitySourceFailureIs500WithoutDetail(t *testing.T) {
	h := activityHandler(t, failingActivity{})
	rec := get(t, h, "GET", activityURL(map[string]string{"handle": goodHandle}), withHeader(testToken))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "secret") {
		t.Errorf("code %d body %q", rec.Code, rec.Body)
	}
}

type failingActivity struct{}

func (failingActivity) Transcript(string, string) (activity.Source, error) {
	return activity.Source{}, errors.New("secret path /home/x")
}

func TestActivityRouteAfterAndNewest(t *testing.T) {
	h := activityHandler(t, &fakeActivity{src: activity.Source{Path: "activity/testdata/main.jsonl"}})
	var first activity.Reply
	rec := get(t, h, "GET", activityURL(map[string]string{"handle": goodHandle}), withHeader(testToken))
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &first) != nil || first.Newest == "" {
		t.Fatalf("first = %d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"newest":"`) {
		t.Errorf("no newest field in %s", rec.Body)
	}
	rec = get(t, h, "GET", activityURL(map[string]string{"handle": goodHandle, "after": first.Newest}), withHeader(testToken))
	var quiet activity.Reply
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &quiet) != nil || len(quiet.Entries) != 0 || quiet.Newest != first.Newest || quiet.Handle != goodHandle {
		t.Errorf("after the newest = %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, h, "GET", activityURL(map[string]string{"handle": goodHandle, "after": "nope"}), withHeader(testToken)); rec.Code != http.StatusBadRequest {
		t.Errorf("bad after = %d", rec.Code)
	}
	if rec := get(t, h, "GET", activityURL(map[string]string{"handle": goodHandle, "after": strings.Repeat("a", 200)}), withHeader(testToken)); rec.Code != http.StatusBadRequest {
		t.Errorf("long after = %d", rec.Code)
	}
}
