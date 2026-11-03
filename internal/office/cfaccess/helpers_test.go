package cfaccess

import (
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/office/cfaccess/cfaccesstest"
)

var testCfg = Config{Host: "clalt.example.com", Team: "team-x", Aud: "aud-1", Email: "me@example.com"}

var t0 = time.Date(2026, 11, 3, 4, 0, 0, 0, time.UTC)

func keys(t *testing.T) (*rsa.PrivateKey, *rsa.PrivateKey) { return cfaccesstest.Keys(t) }

func b64(b []byte) string { return cfaccesstest.B64(b) }

func claims(mod func(map[string]any)) map[string]any {
	return cfaccesstest.Claims("team-x", "aud-1", "me@example.com", t0, mod)
}

func sign(t *testing.T, k *rsa.PrivateKey, kid string, c map[string]any) string {
	return cfaccesstest.Sign(t, k, kid, c)
}

func jwks(pairs map[string]*rsa.PublicKey) []byte { return cfaccesstest.JWKS(pairs) }

// fakeCerts counts fetches and serves whatever body/err is set at the time
type fakeCerts struct {
	body  []byte
	err   error
	calls int
}

func (f *fakeCerts) fetch(url string) ([]byte, error) {
	f.calls++
	if !strings.HasPrefix(url, "https://team-x.cloudflareaccess.com/cdn-cgi/access/certs") {
		return nil, errors.New("unexpected url " + url)
	}
	return f.body, f.err
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }
