package office

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testToken = "0123456789abcdef0123456789abcdef"

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	h, err := NewHandler(Config{
		Token: testToken,
		Port:  4242,
		Hosts: []string{"127.0.0.1:4242", "localhost:4242"},
		Snapshot: func() (Snapshot, error) {
			return Snapshot{V: 1, Accounts: []AccountOut{}, Sessions: []SessionOut{}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// get sends a request with the good host unless the test sets another.
func get(t *testing.T, h http.Handler, method, target string, mod func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	req.Host = "127.0.0.1:4242"
	if mod != nil {
		mod(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func withHeader(tok string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("X-Clalt-Token", tok) }
}

func TestRequestWithNoTokenIsRefused(t *testing.T) {
	for _, target := range []string{"/", "/api/snapshot", "/index.html", "/js/main.js"} {
		if rec := get(t, testHandler(t), "GET", target, nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s with no token = %d, want 401", target, rec.Code)
		}
	}
}

func TestRequestWithAWrongTokenIsRefused(t *testing.T) {
	h := testHandler(t)
	if rec := get(t, h, "GET", "/?t=nope", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong query token = %d", rec.Code)
	}
	if rec := get(t, h, "GET", "/api/snapshot", withHeader(testToken+"0")); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong header token = %d", rec.Code)
	}
	bad := func(r *http.Request) { r.AddCookie(&http.Cookie{Name: cookieNameFor(4242), Value: "nope"}) }
	if rec := get(t, h, "GET", "/api/snapshot", bad); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong cookie = %d", rec.Code)
	}
}

func TestFirstLoadWithTheTokenSetsAStrictCookieAndRedirectsToACleanURL(t *testing.T) {
	rec := get(t, testHandler(t), "GET", "/?t="+testToken, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("code %d location %q, want 303 to /", rec.Code, rec.Header().Get("Location"))
	}
	var c *http.Cookie
	for _, x := range rec.Result().Cookies() {
		if x.Name == cookieNameFor(4242) {
			c = x
		}
	}
	if c == nil || c.Value != testToken || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" {
		t.Errorf("cookie = %+v", c)
	}
}

func TestPageAndSnapshotOpenWithTheCookieOrTheHeader(t *testing.T) {
	h := testHandler(t)
	cookie := func(r *http.Request) { r.AddCookie(&http.Cookie{Name: cookieNameFor(4242), Value: testToken}) }
	for _, mod := range []func(*http.Request){cookie, withHeader(testToken)} {
		if rec := get(t, h, "GET", "/", mod); rec.Code != 200 || !strings.Contains(rec.Body.String(), "<canvas") {
			t.Errorf("page = %d", rec.Code)
		}
		rec := get(t, h, "GET", "/api/snapshot", mod)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"v":1`) {
			t.Errorf("snapshot = %d %s", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("content type %q", ct)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("snapshot must not be cached")
		}
	}
}

func TestOtherHostHeadersAreRefusedEvenWithTheToken(t *testing.T) {
	h := testHandler(t)
	for _, host := range []string{"evil.example", "evil.example:4242", "127.0.0.1", "127.0.0.1:9999", "0.0.0.0:4242", ""} {
		rec := get(t, h, "GET", "/api/snapshot", func(r *http.Request) { r.Host = host; withHeader(testToken)(r) })
		if rec.Code != http.StatusForbidden {
			t.Errorf("host %q = %d, want 403", host, rec.Code)
		}
	}
	for _, host := range []string{"127.0.0.1:4242", "localhost:4242", "LOCALHOST:4242"} {
		rec := get(t, h, "GET", "/api/snapshot", func(r *http.Request) { r.Host = host; withHeader(testToken)(r) })
		if rec.Code != 200 {
			t.Errorf("host %q = %d, want 200", host, rec.Code)
		}
	}
}

func TestOnlyGetAndHeadAreAllowed(t *testing.T) {
	h := testHandler(t)
	for _, m := range []string{"POST", "PUT", "DELETE", "PATCH", "OPTIONS", "TRACE"} {
		rec := get(t, h, m, "/api/snapshot", withHeader(testToken))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s = %d, want 405", m, rec.Code)
		}
		if rec.Header().Get("Allow") != "GET, HEAD" {
			t.Errorf("%s Allow = %q", m, rec.Header().Get("Allow"))
		}
	}
	if rec := get(t, h, "HEAD", "/api/snapshot", withHeader(testToken)); rec.Code != 200 {
		t.Errorf("HEAD = %d", rec.Code)
	}
}

func TestPathsOutsideTheEmbedAre404(t *testing.T) {
	h := testHandler(t)
	for _, p := range []string{"/nope", "/etc/passwd", "/../go.mod", "/js/../../go.mod", "/%2e%2e/go.mod", "/js/", "/js", "/vendor/three/",
		"/api/", "/api/other", "/internal/office/server.go", "/web/index.html", "/index.html/x", "//etc/passwd", "/js/nothing.js"} {
		if rec := get(t, h, "GET", p, withHeader(testToken)); rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", p, rec.Code)
		}
	}
}

func TestEmbeddedFilesAreServedWithTheRightType(t *testing.T) {
	h := testHandler(t)
	cases := map[string]string{"/js/main.js": "javascript", "/index.html": "text/html", "/app.css": "text/css",
		"/assets/furniture/desk.glb": "", "/vendor/three/three.module.min.js": "javascript", "/CREDITS.txt": "text/plain"}
	for p, want := range cases {
		rec := get(t, h, "GET", p, withHeader(testToken))
		if rec.Code != 200 {
			t.Errorf("%s = %d", p, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, want) {
			t.Errorf("%s content type %q, want %q", p, ct, want)
		}
	}
}

func TestEveryResponseCarriesAStrictCSPAndNoCORS(t *testing.T) {
	h := testHandler(t)
	reqs := []struct {
		target string
		mod    func(*http.Request)
	}{
		{"/", withHeader(testToken)}, {"/api/snapshot", withHeader(testToken)}, {"/js/main.js", withHeader(testToken)},
		{"/nope", withHeader(testToken)}, {"/", nil}, {"/?t=" + testToken, nil},
	}
	for _, r := range reqs {
		rec := get(t, h, "GET", r.target, r.mod)
		csp := rec.Header().Get("Content-Security-Policy")
		for _, want := range []string{"default-src 'none'", "script-src 'self'", "style-src 'self'", "img-src 'self'", "connect-src 'self'", "frame-ancestors 'none'", "base-uri 'none'", "form-action 'none'"} {
			if !strings.Contains(csp, want) {
				t.Errorf("%s: CSP %q lacks %q", r.target, csp, want)
			}
		}
		if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") || strings.Contains(csp, "*") {
			t.Errorf("%s: CSP is not strict: %q", r.target, csp)
		}
		for k := range rec.Header() {
			if strings.HasPrefix(strings.ToLower(k), "access-control-") {
				t.Errorf("%s: CORS header %s set", r.target, k)
			}
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Errorf("%s: missing hardening headers", r.target)
		}
	}
}

func TestCSPHashesTheInlineImportMapOfThePage(t *testing.T) {
	csp := get(t, testHandler(t), "GET", "/", withHeader(testToken)).Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "'sha256-") {
		t.Errorf("the page needs an import map, which a script-src of self does not allow without its hash: %q", csp)
	}
}

func TestAFailingSnapshotIs500WithNoDetail(t *testing.T) {
	var logged string
	h, _ := NewHandler(Config{Token: testToken, Hosts: []string{"127.0.0.1:4242"},
		Snapshot: func() (Snapshot, error) { return Snapshot{}, errTest },
		Logf:     func(f string, a ...any) { logged = f },
	})
	rec := get(t, h, "GET", "/api/snapshot", withHeader(testToken))
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("code %d body %q", rec.Code, rec.Body.String())
	}
	if logged == "" {
		t.Errorf("the error was not logged")
	}
}

func TestCookieNameIsPerPortSoTwoRunsDoNotShareOne(t *testing.T) {
	if cookieNameFor(4242) == cookieNameFor(4243) {
		t.Fatal("two ports share a cookie name")
	}
	other, _ := NewHandler(Config{Token: testToken, Port: 4243, Hosts: []string{"127.0.0.1:4243"},
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil }})
	req := httptest.NewRequest("GET", "/api/snapshot", nil)
	req.Host = "127.0.0.1:4243"
	req.AddCookie(&http.Cookie{Name: cookieNameFor(4242), Value: testToken})
	rec := httptest.NewRecorder()
	other.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("a cookie of the other port opened this one: %d", rec.Code)
	}
}

func TestTokenRedirectAlwaysGoesToTheRoot(t *testing.T) {
	for _, target := range []string{"//evil.com/x?t=" + testToken, "/js/main.js?t=" + testToken, "/api/snapshot?t=" + testToken} {
		rec := get(t, testHandler(t), "GET", target, nil)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
			t.Errorf("%s: %d location %q, want 303 to /", target, rec.Code, rec.Header().Get("Location"))
		}
	}
}

func TestCreditsPageIsServedWithAuth(t *testing.T) {
	h := testHandler(t)
	if rec := get(t, h, "GET", "/CREDITS.txt", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("credits without token = %d", rec.Code)
	}
	if rec := get(t, h, "GET", "/", withHeader(testToken)); !strings.Contains(rec.Body.String(), `href="CREDITS.txt"`) {
		t.Error("the page has no credits link")
	}
}
