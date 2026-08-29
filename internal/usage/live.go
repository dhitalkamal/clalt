package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// UsagePath is the live usage endpoint path (same one Claude Code's /usage hits).
const UsagePath = "/api/oauth/usage"

// usageBeta gates the OAuth usage endpoint server-side.
const usageBeta = "oauth-2025-04-20"

// errRateLimited marks a 429 so callers can fall back to cache.
var errRateLimited = errors.New("usage endpoint rate limited")

// IsRateLimited reports whether err is a live-endpoint 429.
func IsRateLimited(err error) bool { return errors.Is(err, errRateLimited) }

// FetchLive queries the authoritative usage endpoint. baseURL is normally
// https://api.anthropic.com; it is a parameter so tests can point at httptest.
// Compliance note: this uses the OAuth token against a non-Claude-Code tool, a
// ToS gray area, which is why the dashboard only calls it behind --live.
func FetchLive(ctx context.Context, hc *http.Client, baseURL, token string) (LiveUsage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+UsagePath, nil)
	if err != nil {
		return LiveUsage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", usageBeta)
	req.Header.Set("Accept", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return LiveUsage{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusTooManyRequests {
		return LiveUsage{}, errRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LiveUsage{}, fmt.Errorf("usage endpoint %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var lu LiveUsage
	if err := json.Unmarshal(body, &lu); err != nil {
		return LiveUsage{}, fmt.Errorf("decode usage response: %w", err)
	}
	return lu, nil
}
