package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ProfilePath is the best-effort identity endpoint. There is no confirmed public
// profile endpoint for subscription OAuth tokens, so callers treat failure as
// non-fatal and fall back to the name the user gave.
const ProfilePath = "/api/oauth/profile"

// Profile is the subset of identity we use to auto-label accounts.
type Profile struct {
	Email        string
	Organization string
}

// profileResponse tolerates both a top-level email and a nested account.email.
type profileResponse struct {
	Email   string `json:"email"`
	Account struct {
		Email string `json:"email"`
	} `json:"account"`
	Organization struct {
		Name string `json:"name"`
	} `json:"organization"`
}

// FetchProfile does a best-effort identity lookup. baseURL is normally
// https://api.anthropic.com; it is a parameter so tests can point at httptest.
func FetchProfile(ctx context.Context, hc *http.Client, baseURL, token string) (Profile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+ProfilePath, nil)
	if err != nil {
		return Profile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", betaHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return Profile{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Profile{}, fmt.Errorf("profile endpoint %d", resp.StatusCode)
	}
	var pr profileResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return Profile{}, err
	}
	email := pr.Email
	if email == "" {
		email = pr.Account.Email
	}
	return Profile{Email: email, Organization: pr.Organization.Name}, nil
}
