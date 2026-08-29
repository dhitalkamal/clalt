package backup

import (
	"bytes"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	plain := []byte(`{"accounts":{"work":{"credential":"secret"}}}`)
	sealed, err := Seal(plain, "correct horse battery staple")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	// sealed output must not contain the plaintext
	if bytes.Contains(sealed, []byte("secret")) {
		t.Error("sealed blob leaks plaintext")
	}
	out, err := Open(sealed, "correct horse battery staple")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(out, plain) {
		t.Errorf("round trip mismatch:\n got %s\nwant %s", out, plain)
	}
}

func TestOpenWrongPassphrase(t *testing.T) {
	sealed, err := Seal([]byte("data"), "right")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(sealed, "wrong"); err == nil {
		t.Error("Open should fail with the wrong passphrase")
	}
}

func TestOpenCorrupt(t *testing.T) {
	if _, err := Open([]byte("garbage"), "x"); err == nil {
		t.Error("Open should fail on a too-short/corrupt blob")
	}
}

func TestBundleRoundTrip(t *testing.T) {
	b := Bundle{Accounts: map[string]Item{
		"work": {Credential: []byte(`{"accessToken":"a"}`), Meta: []byte(`{"email":"w@x.com"}`)},
	}}
	raw, err := b.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := ParseBundle(raw)
	if err != nil {
		t.Fatalf("ParseBundle: %v", err)
	}
	if _, ok := got.Accounts["work"]; !ok {
		t.Error("work account missing after round trip")
	}
}
