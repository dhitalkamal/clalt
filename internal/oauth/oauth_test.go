package oauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAuthorizeURL(t *testing.T) {
	cfg := DefaultConfig()
	raw := cfg.AuthorizeURL("chal123", "state456")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Scheme+"://"+u.Host+u.Path != cfg.AuthEndpoint {
		t.Errorf("endpoint = %q, want %q", u.Scheme+"://"+u.Host+u.Path, cfg.AuthEndpoint)
	}
	q := u.Query()
	checks := map[string]string{
		"code":                  "true",
		"client_id":             cfg.ClientID,
		"response_type":         "code",
		"redirect_uri":          cfg.RedirectURI,
		"scope":                 cfg.Scopes,
		"code_challenge":        "chal123",
		"code_challenge_method": "S256",
		"state":                 "state456",
	}
	for k, want := range checks {
		if got := q.Get(k); got != want {
			t.Errorf("param %q = %q, want %q", k, got, want)
		}
	}
}

func TestConfigEnvOverride(t *testing.T) {
	t.Setenv(EnvClientID, "custom-id")
	t.Setenv(EnvTokenURL, "https://example.test/token")
	cfg := DefaultConfig()
	if cfg.ClientID != "custom-id" {
		t.Errorf("ClientID = %q, want custom-id", cfg.ClientID)
	}
	if cfg.TokenURL != "https://example.test/token" {
		t.Errorf("TokenURL = %q, want override", cfg.TokenURL)
	}
	// unset ones keep defaults
	if cfg.AuthEndpoint != defAuthEndpoint {
		t.Errorf("AuthEndpoint = %q, want default", cfg.AuthEndpoint)
	}
}

func TestParsePastedCode(t *testing.T) {
	code, state := ParsePastedCode("theCode#theState")
	if code != "theCode" || state != "theState" {
		t.Errorf("got (%q,%q)", code, state)
	}
	// no fragment -> whole string is the code, empty state
	code, state = ParsePastedCode("  bareCode  ")
	if code != "bareCode" || state != "" {
		t.Errorf("bare: got (%q,%q)", code, state)
	}
}

func TestExchange(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "sk-ant-oat01-new",
			"refresh_token": "sk-ant-ort01-new",
			"expires_in":    3600,
			"scope":         "user:profile user:inference",
			"token_type":    "Bearer",
		})
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.TokenURL = srv.URL
	c := &Client{HTTP: srv.Client(), Cfg: cfg}
	res, err := c.Exchange(context.Background(), "thecode", "theverifier", "thestate")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	// request carried the right grant fields
	for k, want := range map[string]string{
		"grant_type":    "authorization_code",
		"code":          "thecode",
		"code_verifier": "theverifier",
		"redirect_uri":  cfg.RedirectURI,
		"client_id":     cfg.ClientID,
		"state":         "thestate",
	} {
		if gotBody[k] != want {
			t.Errorf("body[%q] = %v, want %q", k, gotBody[k], want)
		}
	}
	// response parsed correctly
	if res.AccessToken != "sk-ant-oat01-new" || res.RefreshToken != "sk-ant-ort01-new" {
		t.Errorf("tokens wrong: %+v", res)
	}
	if len(res.Scopes) != 2 {
		t.Errorf("scopes = %v", res.Scopes)
	}
	// expiresAt ~= now + 3600s (millis), within a small skew
	wantMs := time.Now().Add(3600 * time.Second).UnixMilli()
	if diff := res.ExpiresAt - wantMs; diff > 5000 || diff < -5000 {
		t.Errorf("expiresAt off by %d ms", diff)
	}
}

func TestRefresh(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "sk-ant-oat01-refreshed",
			"refresh_token": "sk-ant-ort01-rotated",
			"expires_in":    3600,
		})
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.TokenURL = srv.URL
	c := &Client{HTTP: srv.Client(), Cfg: cfg}
	res, err := c.Refresh(context.Background(), "oldrefresh")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if gotBody["grant_type"] != "refresh_token" || gotBody["refresh_token"] != "oldrefresh" {
		t.Errorf("refresh body wrong: %v", gotBody)
	}
	// rotation must be surfaced so the caller can persist it
	if res.RefreshToken != "sk-ant-ort01-rotated" {
		t.Errorf("rotated refresh not returned: %+v", res)
	}
}

func TestExchangeHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()
	cfg := DefaultConfig()
	cfg.TokenURL = srv.URL
	c := &Client{HTTP: srv.Client(), Cfg: cfg}
	_, err := c.Exchange(context.Background(), "bad", "v", "s")
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("expected invalid_grant error, got %v", err)
	}
}
