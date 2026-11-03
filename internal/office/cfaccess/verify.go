package cfaccess

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

// Header is where access puts the signed assertion for the origin.
const Header = "Cf-Access-Jwt-Assertion"

// leeway covers clock drift between this mac and cloudflare
const leeway = 30 * time.Second

// Verifier checks access assertions for one Config. safe for concurrent use.
type Verifier struct {
	cfg   Config
	fetch Fetcher
	now   func() time.Time

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

// NewVerifier builds a Verifier. keys are fetched lazily on first use.
func NewVerifier(cfg Config, fetch Fetcher, now func() time.Time) *Verifier {
	return &Verifier{cfg: cfg, fetch: fetch, now: now, keys: map[string]*rsa.PublicKey{}}
}

// Config is what this Verifier was built for.
func (v *Verifier) Config() Config { return v.cfg }

type claimSet struct {
	Aud   audience `json:"aud"`
	Iss   string   `json:"iss"`
	Email string   `json:"email"`
	Exp   *int64   `json:"exp"`
	Nbf   *int64   `json:"nbf"`
}

// audience is a string or a list of strings, the jwt spec allows both
type audience []string

func (a *audience) UnmarshalJSON(b []byte) error {
	var one string
	if json.Unmarshal(b, &one) == nil {
		*a = audience{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

// Verify says nil only for an RS256 token signed by a current access key for
// this team, meant for this app, in date, and carrying the configured email.
func (v *Verifier) Verify(token string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("not a jwt")
	}
	var head struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodeSegment(parts[0], &head); err != nil {
		return errors.New("bad header")
	}
	// pinning the algorithm closes the alg=none and hs256-with-public-key tricks
	if head.Alg != "RS256" {
		return errors.New("alg must be RS256")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) == 0 {
		return errors.New("bad signature encoding")
	}
	v.mu.Lock()
	pub, err := v.key(head.Kid)
	v.mu.Unlock()
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); err != nil {
		return errors.New("bad signature")
	}
	var c claimSet
	if err := decodeSegment(parts[1], &c); err != nil {
		return errors.New("bad claims")
	}
	return v.checkClaims(c)
}

func (v *Verifier) checkClaims(c claimSet) error {
	now := v.now()
	switch {
	case !contains(c.Aud, v.cfg.Aud):
		return errors.New("wrong audience")
	case c.Iss != v.cfg.Issuer():
		return errors.New("wrong issuer")
	case c.Exp == nil || now.After(time.Unix(*c.Exp, 0).Add(leeway)):
		return errors.New("expired")
	case c.Nbf != nil && now.Add(leeway).Before(time.Unix(*c.Nbf, 0)):
		return errors.New("not valid yet")
	case !strings.EqualFold(c.Email, v.cfg.Email):
		return errors.New("email not allowed")
	}
	return nil
}

func decodeSegment(seg string, into any) error {
	raw, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
