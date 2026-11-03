// Package cfaccesstest signs Cloudflare Access style tokens for tests. it does
// not import cfaccess, so cfaccess's own tests can use it without a cycle.
package cfaccesstest

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"sync"
	"testing"
	"time"
)

var (
	once       sync.Once
	main, spar *rsa.PrivateKey
)

// Keys gives two rsa keys shared by the whole test binary, generation is slow.
func Keys(tb testing.TB) (*rsa.PrivateKey, *rsa.PrivateKey) {
	tb.Helper()
	once.Do(func() {
		main, _ = rsa.GenerateKey(rand.Reader, 2048)
		spar, _ = rsa.GenerateKey(rand.Reader, 2048)
	})
	return main, spar
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// Claims is a valid claim set for team/aud/email around now. mod edits it.
func Claims(team, aud, email string, now time.Time, mod func(map[string]any)) map[string]any {
	c := map[string]any{
		"aud":   []string{aud},
		"iss":   "https://" + team + ".cloudflareaccess.com",
		"email": email,
		"iat":   now.Add(-time.Minute).Unix(),
		"nbf":   now.Add(-time.Minute).Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	if mod != nil {
		mod(c)
	}
	return c
}

// Sign builds an RS256 token for kid, the way access does.
func Sign(tb testing.TB, k *rsa.PrivateKey, kid string, claims map[string]any) string {
	tb.Helper()
	head, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": kid, "typ": "JWT"})
	body, _ := json.Marshal(claims)
	in := b64(head) + "." + b64(body)
	sum := sha256.Sum256([]byte(in))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, sum[:])
	if err != nil {
		tb.Fatal(err)
	}
	return in + "." + b64(sig)
}

// JWKS renders a certs endpoint body for kid -> key.
func JWKS(pairs map[string]*rsa.PublicKey) []byte {
	ks := []map[string]string{}
	for kid, p := range pairs {
		ks = append(ks, map[string]string{
			"kid": kid, "kty": "RSA", "alg": "RS256", "use": "sig",
			"n": b64(p.N.Bytes()), "e": b64(big.NewInt(int64(p.E)).Bytes()),
		})
	}
	out, _ := json.Marshal(map[string]any{"keys": ks})
	return out
}

// B64 is base64url without padding, for tests that hand craft segments.
func B64(b []byte) string { return b64(b) }
