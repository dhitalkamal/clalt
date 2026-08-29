package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"testing"
)

// unreserved chars per RFC 7636 code verifier
var unreserved = regexp.MustCompile(`^[A-Za-z0-9\-._~]+$`)

func TestGeneratePKCE(t *testing.T) {
	p, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	// verifier must be 43-128 chars of unreserved set (RFC 7636)
	if len(p.Verifier) < 43 || len(p.Verifier) > 128 {
		t.Errorf("verifier length %d outside RFC 7636 range", len(p.Verifier))
	}
	if !unreserved.MatchString(p.Verifier) {
		t.Errorf("verifier has disallowed chars: %q", p.Verifier)
	}
	// challenge must be base64url(sha256(verifier)) with no padding
	sum := sha256.Sum256([]byte(p.Verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if p.Challenge != want {
		t.Errorf("challenge = %q, want %q", p.Challenge, want)
	}
}

func TestGeneratePKCEUnique(t *testing.T) {
	a, _ := GeneratePKCE()
	b, _ := GeneratePKCE()
	if a.Verifier == b.Verifier {
		t.Error("two verifiers came back identical")
	}
}

func TestGenerateState(t *testing.T) {
	a, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}
	b, _ := GenerateState()
	if a == "" {
		t.Error("state is empty")
	}
	if a == b {
		t.Error("state values are not unique")
	}
	if !unreserved.MatchString(a) {
		t.Errorf("state has disallowed chars: %q", a)
	}
}
