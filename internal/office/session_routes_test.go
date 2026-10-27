package office

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const runH = "0123456789abcdef"

// fakeActions records the calls and answers with err.
type fakeActions struct {
	calls []string
	err   error
}

func (f *fakeActions) Move(handle, account string) error {
	f.calls = append(f.calls, "move "+handle+" "+account)
	return f.err
}
func (f *fakeActions) Focus(_ context.Context, handle string) error {
	f.calls = append(f.calls, "focus "+handle)
	return f.err
}
func (f *fakeActions) Stop(handle string) error {
	f.calls = append(f.calls, "stop "+handle)
	return f.err
}

func sessionEnv(t *testing.T, a SessionActions) http.Handler {
	t.Helper()
	h, err := NewHandler(Config{
		Token: testToken, Port: 4242, Hosts: []string{"127.0.0.1:4242"}, Actions: a,
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func post(h http.Handler, target, body string, opts ...func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", target, strings.NewReader(body))
	req.Host = "127.0.0.1:4242"
	asCookie(req)
	req.Header.Set("X-Clalt-Edit", "1")
	req.Header.Set("Origin", "http://127.0.0.1:4242")
	req.Header.Set("Content-Type", "application/json")
	for _, o := range opts {
		o(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSessionRoutesHappyPaths(t *testing.T) {
	cases := []struct {
		path, body, call, reply string
		code                    int
	}{
		{"/api/session/move", `{"handle":"` + runH + `","account":"personal"}`, "move " + runH + " personal", `{"queued":true}`, 202},
		{"/api/session/focus", `{"handle":"` + runH + `"}`, "focus " + runH, `{"focused":true}`, 200},
		{"/api/session/stop", `{"handle":"` + runH + `"}`, "stop " + runH, `{"stopping":true}`, 202},
	}
	for _, c := range cases {
		f := &fakeActions{}
		rec := post(sessionEnv(t, f), c.path, c.body)
		if rec.Code != c.code || strings.TrimSpace(rec.Body.String()) != c.reply {
			t.Errorf("%s = %d %s", c.path, rec.Code, rec.Body)
		}
		if len(f.calls) != 1 || f.calls[0] != c.call {
			t.Errorf("%s calls = %v, want %s", c.path, f.calls, c.call)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("%s content type = %q", c.path, ct)
		}
	}
}

var sessionPaths = []string{"/api/session/move", "/api/session/focus", "/api/session/stop"}

const goodBody = `{"handle":"` + runH + `","account":"personal"}`

func TestSessionRoutesNeedEveryGuard(t *testing.T) {
	cases := map[string]struct {
		mod  func(*http.Request)
		code int
	}{
		"no cookie":       {noCookie, 401},
		"no edit header":  {drop("X-Clalt-Edit"), 403},
		"edit header 0":   {set("X-Clalt-Edit", "0"), 403},
		"no origin":       {drop("Origin"), 403},
		"foreign origin":  {set("Origin", "http://evil.example"), 403},
		"other port":      {set("Origin", "http://127.0.0.1:9999"), 403},
		"foreign referer": {func(r *http.Request) { r.Header.Del("Origin"); r.Header.Set("Referer", "http://evil.example/x") }, 403},
		"text/plain":      {set("Content-Type", "text/plain"), 415},
		"no content type": {drop("Content-Type"), 415},
		"foreign host":    {func(r *http.Request) { r.Host = "evil.example" }, 403},
	}
	for name, c := range cases {
		for _, p := range sessionPaths {
			f := &fakeActions{}
			if rec := post(sessionEnv(t, f), p, goodBody, c.mod); rec.Code != c.code {
				t.Errorf("%s %s = %d, want %d", name, p, rec.Code, c.code)
			}
			if len(f.calls) != 0 {
				t.Errorf("%s %s: an action ran: %v", name, p, f.calls)
			}
		}
	}
}

func TestSessionRoutesWantPostOnly(t *testing.T) {
	for _, m := range []string{"GET", "PUT", "DELETE", "HEAD"} {
		req := httptest.NewRequest(m, "/api/session/stop", nil)
		req.Host = "127.0.0.1:4242"
		asCookie(req)
		rec := httptest.NewRecorder()
		sessionEnv(t, &fakeActions{}).ServeHTTP(rec, req)
		if rec.Code != 405 || rec.Header().Get("Allow") != "POST" {
			t.Errorf("%s = %d allow %q", m, rec.Code, rec.Header().Get("Allow"))
		}
	}
}

func TestSessionRoutesRefuseBadBodies(t *testing.T) {
	bodies := map[string]string{
		"not json":           `nope`,
		"no handle":          `{"account":"personal"}`,
		"path handle":        `{"handle":"../etc/passwd","account":"personal"}`,
		"long handle":        `{"handle":"` + strings.Repeat("a", 200) + `","account":"personal"}`,
		"unknown field":      `{"handle":"` + runH + `","account":"personal","pid":1}`,
		"trailing data":      goodBody + `{}`,
		"handle is a number": `{"handle":5,"account":"personal"}`,
	}
	for name, b := range bodies {
		for _, p := range sessionPaths {
			f := &fakeActions{}
			if rec := post(sessionEnv(t, f), p, b); rec.Code != 400 || len(f.calls) != 0 {
				t.Errorf("%s %s = %d calls %v, want 400 and no call", name, p, rec.Code, f.calls)
			}
		}
	}
	f := &fakeActions{}
	if rec := post(sessionEnv(t, f), "/api/session/move", `{"handle":"`+runH+`","account":"a b/c"}`); rec.Code != 400 {
		t.Errorf("bad account = %d", rec.Code)
	}
	if rec := post(sessionEnv(t, f), "/api/session/move", `{"handle":"`+runH+`"}`); rec.Code != 400 {
		t.Errorf("move without account = %d", rec.Code)
	}
	if len(f.calls) != 0 {
		t.Errorf("calls %v", f.calls)
	}
}

func TestSessionRoutesRefuseABodyThatIsTooBig(t *testing.T) {
	big := `{"handle":"` + runH + `","account":"` + strings.Repeat("a", 10000) + `"}`
	if rec := post(sessionEnv(t, &fakeActions{}), "/api/session/move", big); rec.Code != 413 {
		t.Errorf("oversize = %d", rec.Code)
	}
}

func TestSessionRoutesNeverActOnAPlainClaudeSession(t *testing.T) {
	for _, p := range sessionPaths {
		f := &fakeActions{}
		rec := post(sessionEnv(t, f), p, `{"handle":"ext-abcdef012345","account":"personal"}`)
		var out map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if rec.Code != 409 || out["reason"] != outsideWhy || len(f.calls) != 0 {
			t.Errorf("%s = %d %s calls %v", p, rec.Code, rec.Body, f.calls)
		}
	}
}

func TestSessionRoutesAnswer409WithTheReasonOfARefusal(t *testing.T) {
	f := &fakeActions{err: &RefusedError{Reason: "the session has ended"}}
	for _, p := range sessionPaths {
		rec := post(sessionEnv(t, f), p, goodBody)
		if rec.Code != 409 || strings.TrimSpace(rec.Body.String()) != `{"reason":"the session has ended"}` {
			t.Errorf("%s = %d %s", p, rec.Code, rec.Body)
		}
	}
}

func TestSessionRoutesHideAnUnexpectedError(t *testing.T) {
	var logged string
	h, _ := NewHandler(Config{
		Token: testToken, Port: 4242, Hosts: []string{"127.0.0.1:4242"}, Actions: &fakeActions{err: errors.New("open /Users/me/secret: denied")},
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
		Logf:     func(f string, a ...any) { logged = f },
	})
	rec := post(h, "/api/session/stop", goodBody)
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "secret") || logged == "" {
		t.Errorf("= %d %s, logged %q", rec.Code, rec.Body, logged)
	}
}

func TestSessionRoutesAreOffWithoutActionsAndUnknownOnesAre404(t *testing.T) {
	if rec := post(sessionEnv(t, nil), "/api/session/stop", goodBody); rec.Code != 404 {
		t.Errorf("no actions = %d", rec.Code)
	}
	if rec := post(sessionEnv(t, &fakeActions{}), "/api/session/kill", goodBody); rec.Code != 404 {
		t.Errorf("unknown action = %d", rec.Code)
	}
}
