package office

import (
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dhitalkamal/clalt/internal/office/cfaccess"
	"github.com/dhitalkamal/clalt/internal/office/cfaccess/cfaccesstest"
	"github.com/dhitalkamal/clalt/internal/office/layout"
)

const publicHost = "clalt.example.com"

var accessCfg = cfaccess.Config{Host: publicHost, Team: "team-x", Aud: "aud-1", Email: "me@example.com"}

// accessHandler is the office with access on (or off when on is false)
func accessHandler(t *testing.T, on bool) http.Handler {
	t.Helper()
	k, _ := cfaccesstest.Keys(t)
	body := cfaccesstest.JWKS(map[string]*rsa.PublicKey{"k1": &k.PublicKey})
	st := layout.NewStore(t.TempDir(), nil)
	st.Catalog = layout.Catalog{}
	st.Builtin = fstest.MapFS{"default.json": {Data: encode(t, "default")}}
	cfg := Config{
		Token: testToken, Port: 4242, Hosts: []string{"127.0.0.1:4242", "localhost:4242"}, Layouts: st,
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
	}
	if on {
		cfg.Access = cfaccess.NewVerifier(accessCfg, func(string) ([]byte, error) { return body, nil }, time.Now)
	}
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func jwt(t *testing.T, mod func(map[string]any)) string {
	k, _ := cfaccesstest.Keys(t)
	return cfaccesstest.Sign(t, k, "k1", cfaccesstest.Claims("team-x", "aud-1", "me@example.com", time.Now(), mod))
}

// send is one request: host, an optional assertion, then any extra edits
func send(t *testing.T, h http.Handler, method, target, host, assertion, body string, mods ...func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = host
	if assertion != "" {
		req.Header.Set(cfaccess.Header, assertion)
	}
	for _, m := range mods {
		m(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func writeFrom(origin string) func(*http.Request) {
	return func(r *http.Request) {
		r.Header.Set("Origin", origin)
		r.Header.Set(editHeader, "1")
		r.Header.Set("Content-Type", "application/json")
	}
}

func TestPublicHostWithAValidAssertionNeedsNoToken(t *testing.T) {
	h := accessHandler(t, true)
	for _, target := range []string{"/", "/api/snapshot"} {
		if rec := send(t, h, "GET", target, publicHost, jwt(t, nil), ""); rec.Code != 200 {
			t.Errorf("%s = %d, want 200", target, rec.Code)
		}
	}
}

func TestPublicHostIsRefusedWithoutAGoodAssertion(t *testing.T) {
	h := accessHandler(t, true)
	cases := map[string]struct {
		assertion string
		mod       func(*http.Request)
	}{
		"no assertion":            {"", nil},
		"wrong email":             {jwt(t, func(c map[string]any) { c["email"] = "friend@example.com" }), nil},
		"junk":                    {"a.b.c", nil},
		"token only, no access":   {"", withHeader(testToken)},
		"assertion in a cookie":   {"", func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "CF_Authorization", Value: jwt(t, nil)}) }},
		"public host with a port": {jwt(t, nil), func(r *http.Request) { r.Host = publicHost + ":443" }},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var mods []func(*http.Request)
			if c.mod != nil {
				mods = append(mods, c.mod)
			}
			if rec := send(t, h, "GET", "/api/snapshot", publicHost, c.assertion, "", mods...); rec.Code != http.StatusForbidden {
				t.Fatalf("= %d, want 403", rec.Code)
			}
		})
	}
}

// the assertion only counts on the public host, a local request still needs the token
func TestAssertionDoesNotStandInForTheTokenLocally(t *testing.T) {
	h := accessHandler(t, true)
	if rec := send(t, h, "GET", "/api/snapshot", "127.0.0.1:4242", jwt(t, nil), ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("local with assertion only = %d, want 401", rec.Code)
	}
	if rec := send(t, h, "GET", "/api/snapshot", "127.0.0.1:4242", "", "", withHeader(testToken)); rec.Code != 200 {
		t.Fatalf("local with token = %d, want 200", rec.Code)
	}
}

func TestPublicHostIsRefusedWhenAccessIsOff(t *testing.T) {
	h := accessHandler(t, false)
	if rec := send(t, h, "GET", "/api/snapshot", publicHost, jwt(t, nil), ""); rec.Code != http.StatusForbidden {
		t.Fatalf("= %d, want 403", rec.Code)
	}
}

func TestPublicWritesNeedTheAssertionAndThePublicOrigin(t *testing.T) {
	body := string(encode(t, "mine"))
	cases := map[string]struct {
		host, assertion string
		mods            []func(*http.Request)
		want            int
	}{
		"public origin with assertion": {publicHost, jwt(t, nil), []func(*http.Request){writeFrom("https://" + publicHost)}, 200},
		"other origin with assertion":  {publicHost, jwt(t, nil), []func(*http.Request){writeFrom("https://evil.example.com")}, 403},
		"http public origin":           {publicHost, jwt(t, nil), []func(*http.Request){writeFrom("http://" + publicHost)}, 403},
		"public origin, local token":   {"127.0.0.1:4242", "", []func(*http.Request){withHeader(testToken), writeFrom("https://" + publicHost)}, 403},
		"local origin, local token":    {"127.0.0.1:4242", "", []func(*http.Request){withHeader(testToken), writeFrom("http://127.0.0.1:4242")}, 200},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h := accessHandler(t, true)
			if rec := send(t, h, "PUT", "/api/layout", c.host, c.assertion, body, c.mods...); rec.Code != c.want {
				t.Fatalf("= %d %s, want %d", rec.Code, rec.Body, c.want)
			}
		})
	}
}
