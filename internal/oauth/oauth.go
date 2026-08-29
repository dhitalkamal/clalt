package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// These defaults are reverse-engineered from the Claude Code binary and can
// change between releases. Rather than force a recompile when that happens, each
// is overridable via an environment variable (see DefaultConfig). If login or
// refresh starts failing after a Claude Code update, override these first.
const (
	defClientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	defAuthEndpoint = "https://claude.ai/oauth/authorize"
	defTokenURL     = "https://platform.claude.com/v1/oauth/token"
	defRedirectURI  = "https://platform.claude.com/oauth/code/callback"
	defScopes       = "user:profile user:inference user:sessions:claude_code user:mcp_servers"
	// betaHeader gates the OAuth endpoints server-side.
	betaHeader = "oauth-2025-04-20"
)

// env var names for overriding the OAuth defaults.
const (
	EnvClientID     = "CLALT_OAUTH_CLIENT_ID"
	EnvAuthEndpoint = "CLALT_AUTHORIZE_URL"
	EnvTokenURL     = "CLALT_TOKEN_URL"
	EnvRedirectURI  = "CLALT_REDIRECT_URI"
	EnvScopes       = "CLALT_OAUTH_SCOPES"
)

// Config holds the OAuth endpoint/client settings. Build it with DefaultConfig
// so environment overrides are applied.
type Config struct {
	ClientID     string
	AuthEndpoint string
	TokenURL     string
	RedirectURI  string
	Scopes       string
}

// DefaultConfig returns the built-in defaults with any environment overrides
// applied.
func DefaultConfig() Config {
	return Config{
		ClientID:     envOr(EnvClientID, defClientID),
		AuthEndpoint: envOr(EnvAuthEndpoint, defAuthEndpoint),
		TokenURL:     envOr(EnvTokenURL, defTokenURL),
		RedirectURI:  envOr(EnvRedirectURI, defRedirectURI),
		Scopes:       envOr(EnvScopes, defScopes),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// AuthorizeURL builds the browser URL for the manual (code=true) flow.
func (c Config) AuthorizeURL(challenge, state string) string {
	q := url.Values{}
	q.Set("code", "true")
	q.Set("client_id", c.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", c.RedirectURI)
	q.Set("scope", c.Scopes)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	return c.AuthEndpoint + "?" + q.Encode()
}

// TokenResult is the normalized result of an exchange or refresh. Callers must
// persist RefreshToken every time because Claude rotates it on each use.
type TokenResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64 // unix milliseconds
	Scopes       []string
}

// Client talks to the OAuth token endpoint.
type Client struct {
	HTTP *http.Client
	Cfg  Config
}

// NewClient returns a Client with a generous timeout (Claude Code's own 15s cap
// is a known pain point when the endpoint is slow) and env-aware config.
func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 60 * time.Second}, Cfg: DefaultConfig()}
}

// ParsePastedCode splits the value the user copies from the callback page. The
// page renders it as "<code>#<state>"; a bare code yields an empty state.
func ParsePastedCode(pasted string) (code, state string) {
	pasted = strings.TrimSpace(pasted)
	if i := strings.Index(pasted, "#"); i >= 0 {
		return pasted[:i], pasted[i+1:]
	}
	return pasted, ""
}

// Exchange trades an authorization code for tokens.
func (c *Client) Exchange(ctx context.Context, code, verifier, state string) (TokenResult, error) {
	return c.post(ctx, map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  c.Cfg.RedirectURI,
		"client_id":     c.Cfg.ClientID,
		"code_verifier": verifier,
		"state":         state,
	})
}

// Refresh exchanges a refresh token for a fresh (rotated) token set.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (TokenResult, error) {
	return c.post(ctx, map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     c.Cfg.ClientID,
	})
}

// tokenResponse is the raw endpoint payload.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
}

func (c *Client) post(ctx context.Context, body map[string]string) (TokenResult, error) {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Cfg.TokenURL, bytes.NewReader(raw))
	if err != nil {
		return TokenResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-beta", betaHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return TokenResult{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TokenResult{}, fmt.Errorf("token endpoint %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var tr tokenResponse
	if err := json.Unmarshal(data, &tr); err != nil {
		return TokenResult{}, fmt.Errorf("decode token response: %w", err)
	}
	res := TokenResult{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second).UnixMilli(),
	}
	if tr.Scope != "" {
		res.Scopes = strings.Fields(tr.Scope)
	}
	return res, nil
}
