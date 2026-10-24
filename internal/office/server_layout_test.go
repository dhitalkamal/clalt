package office

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// layoutEnv is a handler with a store in a temp dir. the store knows two
// built-in templates, default and loft, and an empty catalog (the layouts
// here have no furniture).
type layoutEnv struct {
	h     http.Handler
	store *layout.Store
}

func newLayoutEnv(t *testing.T, edit bool) layoutEnv {
	t.Helper()
	st := layout.NewStore(t.TempDir(), nil)
	st.Catalog = layout.Catalog{}
	st.Builtin = fstest.MapFS{"default.json": {Data: encode(t, "default")}, "loft.json": {Data: encode(t, "loft")}}
	h, err := NewHandler(Config{
		Token: testToken, Port: 4242, Hosts: []string{"127.0.0.1:4242", "localhost:4242"}, Edit: edit, Layouts: st,
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return layoutEnv{h, st}
}

func encode(t *testing.T, name string) []byte {
	t.Helper()
	l := layout.Minimal()
	l.Name, l.Title = name, strings.ToUpper(name[:1])+name[1:]
	raw, err := layout.Encode(l)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func asCookie(r *http.Request) {
	r.AddCookie(&http.Cookie{Name: cookieNameFor(4242), Value: testToken})
}

// send makes a write request. opts tweak the headers, the good ones are set.
func (e layoutEnv) send(method, target, body string, opts ...func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "127.0.0.1:4242"
	asCookie(req)
	req.Header.Set("X-Clalt-Edit", "1")
	req.Header.Set("Origin", "http://127.0.0.1:4242")
	req.Header.Set("Content-Type", "application/json")
	for _, o := range opts {
		o(req)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func drop(header string) func(*http.Request) { return func(r *http.Request) { r.Header.Del(header) } }
func set(k, v string) func(*http.Request)    { return func(r *http.Request) { r.Header.Set(k, v) } }
func noCookie(r *http.Request)               { r.Header.Del("Cookie") }

type reply struct {
	Edit   bool
	Layout layout.Layout
}

func decodeReply(t *testing.T, rec *httptest.ResponseRecorder) reply {
	t.Helper()
	var r reply
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("%v in %s", err, rec.Body.String())
	}
	return r
}

func TestGetLayoutAlwaysAnswersWithTheActiveLayoutAndTheEditFlag(t *testing.T) {
	for _, edit := range []bool{false, true} {
		e := newLayoutEnv(t, edit)
		rec := e.send("GET", "/api/layout", "")
		if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("edit=%v: %d %s", edit, rec.Code, rec.Body)
		}
		if r := decodeReply(t, rec); r.Edit != edit || r.Layout.Name != "default" || r.Layout.V != 1 {
			t.Errorf("edit=%v: reply %+v", edit, r)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Error("layout may be cached")
		}
	}
}

func TestGetLayoutNeedsTheToken(t *testing.T) {
	e := newLayoutEnv(t, true)
	if rec := e.send("GET", "/api/layout", "", noCookie); rec.Code != 401 {
		t.Errorf("no cookie = %d", rec.Code)
	}
}

func TestGetLayoutWithoutAStoreServesTheDefault(t *testing.T) {
	rec := get(t, testHandler(t), "GET", "/api/layout", withHeader(testToken))
	if rec.Code != 200 || decodeReply(t, rec).Layout.Name != "default" {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
}

func TestWithoutEditTheWriteRoutesAre404(t *testing.T) {
	e := newLayoutEnv(t, false)
	for _, c := range [][2]string{{"PUT", "/api/layout"}, {"GET", "/api/layouts"}, {"POST", "/api/layout/use"}, {"POST", "/api/layouts"}, {"DELETE", "/api/layout"}, {"GET", "/api/layout/use"}} {
		body := string(encode(t, "mine"))
		if rec := e.send(c[0], c[1], body); rec.Code != 404 {
			t.Errorf("%s %s = %d, want 404", c[0], c[1], rec.Code)
		}
	}
	if _, err := e.store.Load("mine"); err == nil {
		t.Error("a layout was saved")
	}
}

func TestEditRoutesNeedTheTokenFirst(t *testing.T) {
	for _, edit := range []bool{false, true} {
		e := newLayoutEnv(t, edit)
		if rec := e.send("PUT", "/api/layout", string(encode(t, "mine")), noCookie); rec.Code != 401 {
			t.Errorf("edit=%v: PUT without cookie = %d", edit, rec.Code)
		}
	}
}

func TestWritesNeedEveryGuard(t *testing.T) {
	body := string(encode(t, "mine"))
	cases := map[string]func(*http.Request){
		"no edit header":  drop("X-Clalt-Edit"),
		"edit header 0":   set("X-Clalt-Edit", "0"),
		"no origin":       drop("Origin"),
		"foreign origin":  set("Origin", "http://evil.example"),
		"other port":      set("Origin", "http://127.0.0.1:9999"),
		"https origin":    set("Origin", "https://127.0.0.1:4242"),
		"null origin":     set("Origin", "null"),
		"prefix origin":   set("Origin", "http://127.0.0.1:4242.evil.example"),
		"foreign referer": func(r *http.Request) { r.Header.Del("Origin"); r.Header.Set("Referer", "http://evil.example/x") },
		"origin wins": func(r *http.Request) {
			r.Header.Set("Origin", "http://evil.example")
			r.Header.Set("Referer", "http://127.0.0.1:4242/")
		},
	}
	for name, mod := range cases {
		e := newLayoutEnv(t, true)
		if rec := e.send("PUT", "/api/layout", body, mod); rec.Code != 403 {
			t.Errorf("%s: PUT = %d, want 403", name, rec.Code)
		}
		if rec := e.send("POST", "/api/layout/use", `{"name":"loft"}`, mod); rec.Code != 403 {
			t.Errorf("%s: POST use = %d, want 403", name, rec.Code)
		}
		if e.store.ActiveName() != "default" {
			t.Errorf("%s: the active layout changed", name)
		}
		if _, err := e.store.Load("mine"); err == nil {
			t.Errorf("%s: a layout was saved", name)
		}
	}
}

func TestWritesRefuseTheWrongContentTypeAndAnOversizeBody(t *testing.T) {
	e := newLayoutEnv(t, true)
	body := string(encode(t, "mine"))
	if rec := e.send("PUT", "/api/layout", body, set("Content-Type", "text/plain")); rec.Code != 415 {
		t.Errorf("text/plain = %d", rec.Code)
	}
	if rec := e.send("PUT", "/api/layout", body, drop("Content-Type")); rec.Code != 415 {
		t.Errorf("no content type = %d", rec.Code)
	}
	big := body[:len(body)-2] + `,"pad":"` + strings.Repeat("a", layout.MaxBytes) + `"}`
	if rec := e.send("PUT", "/api/layout", big); rec.Code != 413 {
		t.Errorf("oversize = %d", rec.Code)
	}
}

func TestEditRoutesRefuseOtherMethods(t *testing.T) {
	e := newLayoutEnv(t, true)
	for _, c := range [][2]string{{"POST", "/api/layout"}, {"DELETE", "/api/layout"}, {"PUT", "/api/layouts"}, {"GET", "/api/layout/use"}, {"PUT", "/api/layout/use"}} {
		if rec := e.send(c[0], c[1], "{}"); rec.Code != 405 {
			t.Errorf("%s %s = %d, want 405", c[0], c[1], rec.Code)
		}
	}
}

func TestPutRefusesBadLayouts(t *testing.T) {
	e := newLayoutEnv(t, true)
	for name, body := range map[string]string{
		"junk":        `nope`,
		"unknown key": `{"v":1,"extra":1}`,
		"invalid":     strings.Replace(string(encode(t, "mine")), `"w": 8`, `"w": 0`, 1),
	} {
		rec := e.send("PUT", "/api/layout", body)
		if rec.Code != 400 || strings.Contains(rec.Body.String(), "/") && strings.Contains(rec.Body.String(), e.store.Root) {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
}

func TestPutOverABuiltInSavesACustomCopyAndSwitchesToIt(t *testing.T) {
	e := newLayoutEnv(t, true)
	l := layout.Minimal()
	l.Title = "Changed"
	l.Name = "ignored-name"
	rec := e.send("PUT", "/api/layout", string(mustEncode(t, l)))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	r := decodeReply(t, rec)
	if !r.Edit || r.Layout.Name != "default-custom" || r.Layout.Title != "Changed" {
		t.Errorf("reply %+v", r)
	}
	if e.store.ActiveName() != "default-custom" {
		t.Errorf("active = %q", e.store.ActiveName())
	}
	if got, err := e.store.Load("default"); err != nil || got.Title != "Default" {
		t.Errorf("the built-in changed: %v %q", err, got.Title)
	}
	rec = e.send("GET", "/api/layout", "")
	if decodeReply(t, rec).Layout.Title != "Changed" {
		t.Error("GET does not show the saved layout")
	}
}

func TestPutOverAUserLayoutSavesUnderItsName(t *testing.T) {
	e := newLayoutEnv(t, true)
	mine := layout.Minimal()
	mine.Name = "mine"
	if err := e.store.Save(mine); err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetActive("mine"); err != nil {
		t.Fatal(err)
	}
	mine.Title = "Edited"
	if rec := e.send("PUT", "/api/layout", string(mustEncode(t, mine))); rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got, _ := e.store.Load("mine"); got.Title != "Edited" || e.store.ActiveName() != "mine" {
		t.Errorf("title %q active %q", got.Title, e.store.ActiveName())
	}
}

func TestPutAcceptsLocalhostOriginOrAMatchingReferer(t *testing.T) {
	body := string(encode(t, "mine"))
	for name, mod := range map[string]func(*http.Request){
		"localhost": set("Origin", "http://localhost:4242"),
		"referer": func(r *http.Request) {
			r.Header.Del("Origin")
			r.Header.Set("Referer", "http://127.0.0.1:4242/some/page")
		},
	} {
		e := newLayoutEnv(t, true)
		if rec := e.send("PUT", "/api/layout", body, mod); rec.Code != 200 {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
}

func mustEncode(t *testing.T, l layout.Layout) []byte {
	t.Helper()
	raw, err := layout.Encode(l)
	if err != nil {
		t.Fatal(fmt.Sprint(err))
	}
	return raw
}
