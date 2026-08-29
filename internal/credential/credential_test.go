package credential

import (
	"encoding/json"
	"testing"
	"time"
)

// this is the exact shape Claude Code writes under the claudeAiOauth key
const sample = `{
  "accessToken": "sk-ant-oat01-abc",
  "refreshToken": "sk-ant-ort01-def",
  "expiresAt": 1999999999000,
  "scopes": ["user:profile", "user:inference"],
  "subscriptionType": "max"
}`

func TestUnmarshalMatchesClaudeShape(t *testing.T) {
	var c Credential
	if err := json.Unmarshal([]byte(sample), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.AccessToken != "sk-ant-oat01-abc" {
		t.Errorf("accessToken = %q", c.AccessToken)
	}
	if c.RefreshToken != "sk-ant-ort01-def" {
		t.Errorf("refreshToken = %q", c.RefreshToken)
	}
	if c.ExpiresAt != 1999999999000 {
		t.Errorf("expiresAt = %d", c.ExpiresAt)
	}
	if len(c.Scopes) != 2 || c.SubscriptionType != "max" {
		t.Errorf("scopes/subscription wrong: %+v", c)
	}
}

func TestRoundTripKeepsJSONKeys(t *testing.T) {
	var c Credential
	_ = json.Unmarshal([]byte(sample), &c)
	out, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// keys must survive so Claude Code can read what we write back
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	for _, k := range []string{"accessToken", "refreshToken", "expiresAt", "scopes", "subscriptionType"} {
		if _, ok := m[k]; !ok {
			t.Errorf("marshaled JSON missing key %q", k)
		}
	}
}

func TestIsExpired(t *testing.T) {
	now := time.Now()
	// expires 30s from now, but 60s buffer -> treated as expired
	soon := Credential{ExpiresAt: now.Add(30 * time.Second).UnixMilli()}
	if !soon.IsExpired(60 * time.Second) {
		t.Error("token expiring within buffer should be expired")
	}
	// expires 10 min out -> not expired
	later := Credential{ExpiresAt: now.Add(10 * time.Minute).UnixMilli()}
	if later.IsExpired(60 * time.Second) {
		t.Error("token 10m out should not be expired")
	}
	// zero expiry -> treat as expired (unknown)
	if !(Credential{}).IsExpired(0) {
		t.Error("zero expiresAt should be expired")
	}
}
