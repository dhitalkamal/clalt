package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCE holds a code verifier and its S256 challenge (RFC 7636).
type PKCE struct {
	Verifier  string
	Challenge string
}

// GeneratePKCE creates a fresh verifier/challenge pair using S256.
func GeneratePKCE() (PKCE, error) {
	// 32 random bytes -> 43-char base64url verifier, inside the RFC range
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return PKCE{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	return PKCE{
		Verifier:  verifier,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
	}, nil
}

// GenerateState returns a random anti-CSRF state value in the unreserved charset.
func GenerateState() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
