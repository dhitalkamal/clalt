package office

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeAutostart struct {
	supported, enabled bool
	err                error
	sets               []bool
}

func (f *fakeAutostart) Status() AutostartStatus {
	return AutostartStatus{Supported: f.supported, Enabled: f.enabled}
}

func (f *fakeAutostart) Set(on bool) error {
	f.sets = append(f.sets, on)
	if f.err == nil {
		f.enabled = on
	}
	return f.err
}

func autostartHandler(t *testing.T, a Autostarter) http.Handler {
	t.Helper()
	h, err := NewHandler(Config{
		Token: testToken, Port: 4242, Hosts: []string{"127.0.0.1:4242"}, Autostart: a, Version: "9.9.9",
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func autoReq(h http.Handler, method, body string, opts ...func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/office/autostart", strings.NewReader(body))
	req.Host = "127.0.0.1:4242"
	req.AddCookie(&http.Cookie{Name: cookieNameFor(4242), Value: testToken})
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

func TestAutostartGet(t *testing.T) {
	f := &fakeAutostart{supported: true, enabled: true}
	rec := autoReq(autostartHandler(t, f), "GET", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"supported":true,"enabled":true}` {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
	if rec := autoReq(autostartHandler(t, f), "GET", "", func(r *http.Request) { r.Header.Del("Cookie") }); rec.Code != 401 {
		t.Errorf("no token = %d", rec.Code)
	}
}

func TestAutostartPostSetsAndReplies(t *testing.T) {
	f := &fakeAutostart{supported: true}
	rec := autoReq(autostartHandler(t, f), "POST", `{"enabled":true}`)
	if rec.Code != 200 || !f.enabled || !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Errorf("%d %s %+v", rec.Code, rec.Body, f)
	}
	autoReq(autostartHandler(t, f), "POST", `{"enabled":false}`)
	if f.enabled || len(f.sets) != 2 {
		t.Errorf("%+v", f)
	}
}

func TestAutostartPostGuards(t *testing.T) {
	f := &fakeAutostart{supported: true}
	h := autostartHandler(t, f)
	set := func(k, v string) func(*http.Request) { return func(r *http.Request) { r.Header.Set(k, v) } }
	drop := func(k string) func(*http.Request) { return func(r *http.Request) { r.Header.Del(k) } }
	cases := map[string]struct {
		opt  func(*http.Request)
		want int
	}{
		"no token":       {drop("Cookie"), 401},
		"no edit header": {drop("X-Clalt-Edit"), 403},
		"foreign origin": {set("Origin", "http://evil.example"), 403},
		"no origin":      {drop("Origin"), 403},
		"text body":      {set("Content-Type", "text/plain"), 415},
		"bad host":       {func(r *http.Request) { r.Host = "evil.example" }, 403},
	}
	for name, c := range cases {
		if rec := autoReq(h, "POST", `{"enabled":true}`, c.opt); rec.Code != c.want {
			t.Errorf("%s = %d, want %d", name, rec.Code, c.want)
		}
	}
	for _, m := range []string{"PUT", "DELETE", "PATCH"} {
		if rec := autoReq(h, m, `{"enabled":true}`); rec.Code != 405 {
			t.Errorf("%s = %d", m, rec.Code)
		}
	}
	for _, body := range []string{``, `nope`, `{"enabled":"yes"}`, `{"enabled":true,"x":1}`, `{}`} {
		if rec := autoReq(h, "POST", body); rec.Code != 400 {
			t.Errorf("%q = %d", body, rec.Code)
		}
	}
	if len(f.sets) != 0 {
		t.Errorf("a refused request changed it: %v", f.sets)
	}
}

func TestAutostartUnsupportedAndFailing(t *testing.T) {
	f := &fakeAutostart{supported: false}
	rec := autoReq(autostartHandler(t, f), "POST", `{"enabled":true}`)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "macOS only") || len(f.sets) != 0 {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
	g := &fakeAutostart{supported: true, err: errors.New("launchctl exploded at /secret/path")}
	rec = autoReq(autostartHandler(t, g), "POST", `{"enabled":true}`)
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "secret") {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
}

func TestAutostartWithoutServiceIs404(t *testing.T) {
	if rec := autoReq(autostartHandler(t, nil), "GET", ""); rec.Code != 404 {
		t.Errorf("%d", rec.Code)
	}
}

func TestPingNeedsTheTokenAndNamesThePid(t *testing.T) {
	h := autostartHandler(t, nil)
	req := httptest.NewRequest("GET", "/api/ping", nil)
	req.Host = "127.0.0.1:4242"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("no token = %d", rec.Code)
	}
	req.Header.Set("X-Clalt-Token", testToken)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"app":"clalt-office"`) || !strings.Contains(rec.Body.String(), `"pid":`) || !strings.Contains(rec.Body.String(), `"version":"9.9.9"`) {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
}
