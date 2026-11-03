package cfaccess

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"time"
)

// Fetcher gets the body of a url. tests swap in a fake, the office uses HTTPFetch.
type Fetcher func(url string) ([]byte, error)

// refetchGap stops a stream of unknown kids from turning into a request flood
const refetchGap = time.Minute

var httpClient = &http.Client{Timeout: 5 * time.Second}

// HTTPFetch is the real Fetcher. the certs document is small, 1 MiB is plenty.
func HTTPFetch(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("certs: " + resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// key looks kid up, refetching the certs when it is unknown and the last
// fetch is old enough. callers hold v.mu.
func (v *Verifier) key(kid string) (*rsa.PublicKey, error) {
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	now := v.now()
	if !v.fetched.IsZero() && now.Sub(v.fetched) < refetchGap {
		return nil, errors.New("unknown key id")
	}
	v.fetched = now
	raw, err := v.fetch(v.cfg.certsURL())
	if err != nil {
		return nil, err
	}
	ks, err := parseJWKS(raw)
	if err != nil {
		return nil, err
	}
	// a failed fetch keeps the old keys, a good one replaces them so revoked keys go
	v.keys = ks
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	return nil, errors.New("unknown key id")
}

func parseJWKS(raw []byte) (map[string]*rsa.PublicKey, error) {
	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	out := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil || len(e) == 0 || len(e) > 4 {
			continue
		}
		pub := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
		if pub.N.BitLen() < 2048 {
			continue
		}
		out[k.Kid] = pub
	}
	if len(out) == 0 {
		return nil, errors.New("certs: no usable rsa keys")
	}
	return out, nil
}
