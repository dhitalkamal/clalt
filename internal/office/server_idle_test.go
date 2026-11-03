package office

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

const idleDefaults = `{"default":"break","teamDefault":"desk","accounts":{},"teams":{},"subtypes":{}}`

type idleEnv struct {
	h    http.Handler
	dir  string
	logs *[]string
}

func newIdleEnv(t *testing.T) idleEnv {
	t.Helper()
	dir := t.TempDir()
	var logs []string
	h, err := NewHandler(Config{
		Token: testToken, Port: 4242, Hosts: []string{"127.0.0.1:4242"}, Layouts: layout.NewStore(dir, nil),
		Logf:     func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) },
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return idleEnv{h, dir, &logs}
}

func (e idleEnv) do(method, body string, opts ...func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/idle", strings.NewReader(body))
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

func TestIdleGetDefaultsWithoutAFile(t *testing.T) {
	rec := newIdleEnv(t).do("GET", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != idleDefaults {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
}

func TestIdleGetNeedsTheToken(t *testing.T) {
	if rec := newIdleEnv(t).do("GET", "", func(r *http.Request) { r.Header.Del("Cookie") }); rec.Code != 401 {
		t.Errorf("no token = %d", rec.Code)
	}
}

func TestIdlePostRoundTripAndMode(t *testing.T) {
	e := newIdleEnv(t)
	body := `{"default":"hide","teamDefault":"room:lounge","accounts":{"work":"desk"},"teams":{"my team":"break"},"subtypes":{"explore":"room:a-1"}}`
	rec := e.do("POST", body)
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != body {
		t.Fatalf("post %d %s", rec.Code, rec.Body)
	}
	if got := e.do("GET", ""); strings.TrimSpace(got.Body.String()) != body {
		t.Errorf("get = %s", got.Body)
	}
	fi, err := os.Stat(filepath.Join(e.dir, "idle.json"))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("file %v %v", fi, err)
	}
}

func TestIdleBadFileAnswersDefaultsAndLogs(t *testing.T) {
	e := newIdleEnv(t)
	for _, raw := range []string{`not json`, `{"default":"nope","teamDefault":"desk"}`} {
		if err := os.WriteFile(filepath.Join(e.dir, "idle.json"), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		*e.logs = nil
		rec := e.do("GET", "")
		if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != idleDefaults {
			t.Errorf("%q: %d %s", raw, rec.Code, rec.Body)
		}
		if len(*e.logs) != 1 || !strings.Contains((*e.logs)[0], "idle") {
			t.Errorf("%q: logs %v", raw, *e.logs)
		}
	}
}

func TestIdlePostRejectsBadBodies(t *testing.T) {
	e := newIdleEnv(t)
	many := func(n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, `%q:"desk",`, fmt.Sprint("t", i))
		}
		return strings.TrimSuffix(b.String(), ",")
	}
	cases := map[string]string{
		"bad choice":       `{"default":"sleep","teamDefault":"desk"}`,
		"bad room id":      `{"default":"room:Big Room","teamDefault":"desk"}`,
		"empty room id":    `{"default":"room:","teamDefault":"desk"}`,
		"bad map choice":   `{"default":"desk","teamDefault":"desk","teams":{"a":"x"}}`,
		"unknown field":    `{"default":"desk","teamDefault":"desk","extra":1}`,
		"missing default":  `{"teamDefault":"desk"}`,
		"missing team def": `{"default":"desk"}`,
		"bad account key":  `{"default":"desk","teamDefault":"desk","accounts":{"a b":"desk"}}`,
		"empty team key":   `{"default":"desk","teamDefault":"desk","teams":{"":"desk"}}`,
		"control key":      `{"default":"desk","teamDefault":"desk","subtypes":{"a\u0001":"desk"}}`,
		"non ascii key":    `{"default":"desk","teamDefault":"desk","subtypes":{"café":"desk"}}`,
		"long key":         `{"default":"desk","teamDefault":"desk","teams":{"` + strings.Repeat("a", 65) + `":"desk"}}`,
		"too many":         `{"default":"desk","teamDefault":"desk","teams":{` + many(201) + `}}`,
		"trailing data":    `{"default":"desk","teamDefault":"desk"} {}`,
		"not json":         `nope`,
	}
	for name, body := range cases {
		rec := e.do("POST", body)
		if rec.Code != 400 || strings.Count(strings.TrimSpace(rec.Body.String()), "\n") != 0 {
			t.Errorf("%s = %d %q", name, rec.Code, rec.Body)
		}
	}
	if _, err := os.Stat(filepath.Join(e.dir, "idle.json")); err == nil {
		t.Error("a refused body wrote the file")
	}
	if rec := e.do("POST", `{"default":"desk","teamDefault":"desk","teams":{`+many(200)+`}}`); rec.Code != 200 {
		t.Errorf("200 entries = %d %s", rec.Code, rec.Body)
	}
}

func TestIdlePostGuards(t *testing.T) {
	e := newIdleEnv(t)
	ok := `{"default":"desk","teamDefault":"desk"}`
	set := func(k, v string) func(*http.Request) { return func(r *http.Request) { r.Header.Set(k, v) } }
	drop := func(k string) func(*http.Request) { return func(r *http.Request) { r.Header.Del(k) } }
	cases := map[string]struct {
		opt  func(*http.Request)
		want int
	}{
		"no token":       {drop("Cookie"), 401},
		"no edit header": {drop("X-Clalt-Edit"), 403},
		"foreign origin": {set("Origin", "http://evil.example"), 403},
		"text body":      {set("Content-Type", "text/plain"), 415},
	}
	for name, c := range cases {
		if rec := e.do("POST", ok, c.opt); rec.Code != c.want {
			t.Errorf("%s = %d, want %d", name, rec.Code, c.want)
		}
	}
	for _, m := range []string{"PUT", "DELETE", "PATCH"} {
		if rec := e.do(m, ok); rec.Code != 405 {
			t.Errorf("%s = %d", m, rec.Code)
		}
	}
	big := `{"default":"desk","teamDefault":"desk","pad":"` + strings.Repeat("a", layout.MaxBytes) + `"}`
	if rec := e.do("POST", big); rec.Code != 413 {
		t.Errorf("oversize = %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(e.dir, "idle.json")); err == nil {
		t.Error("a refused request wrote the file")
	}
}

func TestIdleWithoutAStoreServesDefaultsAndRefusesWrites(t *testing.T) {
	h, err := NewHandler(Config{
		Token: testToken, Port: 4242, Hosts: []string{"127.0.0.1:4242"},
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	e := idleEnv{h: h}
	if rec := e.do("GET", ""); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != idleDefaults {
		t.Errorf("get %d %s", rec.Code, rec.Body)
	}
	if rec := e.do("POST", `{"default":"desk","teamDefault":"desk"}`); rec.Code != 404 {
		t.Errorf("post = %d", rec.Code)
	}
}
