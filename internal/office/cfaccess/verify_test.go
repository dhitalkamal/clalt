package cfaccess

import (
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T) (*Verifier, *fakeCerts, *clock) {
	k, _ := keys(t)
	f := &fakeCerts{body: jwks(map[string]*rsa.PublicKey{"k1": &k.PublicKey})}
	c := &clock{t: t0}
	return NewVerifier(testCfg, f.fetch, c.now), f, c
}

func TestVerifyAcceptsAGoodToken(t *testing.T) {
	v, _, _ := setup(t)
	k, _ := keys(t)
	if err := v.Verify(sign(t, k, "k1", claims(nil))); err != nil {
		t.Fatalf("good token refused: %v", err)
	}
}

func TestVerifyMatchesEmailIgnoringCase(t *testing.T) {
	v, _, _ := setup(t)
	k, _ := keys(t)
	tok := sign(t, k, "k1", claims(func(c map[string]any) { c["email"] = "Me@Example.COM" }))
	if err := v.Verify(tok); err != nil {
		t.Fatalf("email case should not matter: %v", err)
	}
}

func TestVerifyRefusesBadClaims(t *testing.T) {
	cases := map[string]func(map[string]any){
		"wrong aud":      func(c map[string]any) { c["aud"] = []string{"other"} },
		"no aud":         func(c map[string]any) { delete(c, "aud") },
		"wrong iss":      func(c map[string]any) { c["iss"] = "https://evil.cloudflareaccess.com" },
		"wrong email":    func(c map[string]any) { c["email"] = "friend@example.com" },
		"no email":       func(c map[string]any) { delete(c, "email") },
		"expired":        func(c map[string]any) { c["exp"] = t0.Add(-2 * time.Minute).Unix() },
		"no exp":         func(c map[string]any) { delete(c, "exp") },
		"not yet valid":  func(c map[string]any) { c["nbf"] = t0.Add(2 * time.Minute).Unix() },
		"aud as a plain": func(c map[string]any) { c["aud"] = "other" },
	}
	k, _ := keys(t)
	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			v, _, _ := setup(t)
			if err := v.Verify(sign(t, k, "k1", claims(mod))); err == nil {
				t.Fatal("token accepted")
			}
		})
	}
}

func TestVerifyAcceptsAudAsAString(t *testing.T) {
	v, _, _ := setup(t)
	k, _ := keys(t)
	if err := v.Verify(sign(t, k, "k1", claims(func(c map[string]any) { c["aud"] = "aud-1" }))); err != nil {
		t.Fatalf("single string aud refused: %v", err)
	}
}

func TestVerifyRefusesAnotherKeysSignature(t *testing.T) {
	v, _, _ := setup(t)
	_, other := keys(t)
	if err := v.Verify(sign(t, other, "k1", claims(nil))); err == nil {
		t.Fatal("token signed by a key we do not trust was accepted")
	}
}

func TestVerifyRefusesOtherAlgorithms(t *testing.T) {
	v, _, _ := setup(t)
	k, _ := keys(t)
	good := sign(t, k, "k1", claims(nil))
	parts := strings.Split(good, ".")
	for _, alg := range []string{"none", "HS256", "RS512"} {
		head := b64([]byte(`{"alg":"` + alg + `","kid":"k1"}`))
		for _, tok := range []string{head + "." + parts[1] + "." + parts[2], head + "." + parts[1] + "."} {
			if err := v.Verify(tok); err == nil {
				t.Fatalf("alg %s accepted", alg)
			}
		}
	}
}

func TestVerifyRefusesJunk(t *testing.T) {
	v, _, _ := setup(t)
	for _, tok := range []string{"", "a.b", "a.b.c.d", "!!.!!.!!", "e30.e30.e30"} {
		if err := v.Verify(tok); err == nil {
			t.Fatalf("junk %q accepted", tok)
		}
	}
}

func TestVerifyCachesKeys(t *testing.T) {
	v, f, _ := setup(t)
	k, _ := keys(t)
	for i := 0; i < 3; i++ {
		if err := v.Verify(sign(t, k, "k1", claims(nil))); err != nil {
			t.Fatal(err)
		}
	}
	if f.calls != 1 {
		t.Fatalf("certs fetched %d times, want 1", f.calls)
	}
}

func TestVerifyRefetchesForANewKidAtMostOncePerMinute(t *testing.T) {
	v, f, c := setup(t)
	k, other := keys(t)
	_ = v.Verify(sign(t, k, "k1", claims(nil)))
	// a rotated key shows up at cloudflare
	f.body = jwks(map[string]*rsa.PublicKey{"k1": &k.PublicKey, "k2": &other.PublicKey})
	if err := v.Verify(sign(t, other, "k2", claims(nil))); err == nil {
		t.Fatal("refetch happened inside the minute")
	}
	c.t = c.t.Add(61 * time.Second)
	if err := v.Verify(sign(t, other, "k2", claims(func(m map[string]any) { m["exp"] = c.t.Add(time.Hour).Unix() }))); err != nil {
		t.Fatalf("rotated key not picked up: %v", err)
	}
	if f.calls != 2 {
		t.Fatalf("certs fetched %d times, want 2", f.calls)
	}
}

func TestVerifyFailsClosedWhenCertsCannotBeFetched(t *testing.T) {
	v, f, c := setup(t)
	k, _ := keys(t)
	f.err = errors.New("offline")
	if err := v.Verify(sign(t, k, "k1", claims(nil))); err == nil {
		t.Fatal("accepted with no keys")
	}
	f.err = nil
	c.t = c.t.Add(61 * time.Second)
	tok := sign(t, k, "k1", claims(func(m map[string]any) { m["exp"] = c.t.Add(time.Hour).Unix() }))
	if err := v.Verify(tok); err != nil {
		t.Fatalf("did not recover once certs came back: %v", err)
	}
}

func TestVerifyIgnoresKeysThatAreNotRSA(t *testing.T) {
	v, f, _ := setup(t)
	f.body = []byte(`{"keys":[{"kid":"k1","kty":"EC","crv":"P-256","x":"AA","y":"AA"}]}`)
	k, _ := keys(t)
	if err := v.Verify(sign(t, k, "k1", claims(nil))); err == nil {
		t.Fatal("accepted against a non rsa key")
	}
}
