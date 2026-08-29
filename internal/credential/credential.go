// Package credential models the OAuth blob that Claude Code stores under the
// claudeAiOauth key, in both the macOS Keychain and ~/.claude/.credentials.json.
package credential

import "time"

// Credential mirrors the claudeAiOauth object exactly so we can read what Claude
// Code wrote and write back something it can read.
type Credential struct {
	AccessToken      string   `json:"accessToken"`
	RefreshToken     string   `json:"refreshToken"`
	ExpiresAt        int64    `json:"expiresAt"` // unix milliseconds
	Scopes           []string `json:"scopes"`
	SubscriptionType string   `json:"subscriptionType"`
}

// IsExpired reports whether the token is expired, treating anything within buffer
// of expiry (or with no expiry recorded) as expired so callers refresh early.
func (c Credential) IsExpired(buffer time.Duration) bool {
	if c.ExpiresAt == 0 {
		return true
	}
	expiry := time.UnixMilli(c.ExpiresAt)
	return time.Now().Add(buffer).After(expiry)
}

// ExpiresAtTime returns the expiry as a time.Time (zero if unset).
func (c Credential) ExpiresAtTime() time.Time {
	if c.ExpiresAt == 0 {
		return time.Time{}
	}
	return time.UnixMilli(c.ExpiresAt)
}
