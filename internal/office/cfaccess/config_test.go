package cfaccess

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "access.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadConfigIsOffWithoutAFile(t *testing.T) {
	_, on, err := LoadConfig(t.TempDir())
	if err != nil || on {
		t.Fatalf("on=%v err=%v, want off and no error", on, err)
	}
}

func TestLoadConfigReadsAndNormalises(t *testing.T) {
	dir := writeConfig(t, `{"host":"Clalt.Example.com","team":"team-x","aud":"aud-1","email":"Me@Example.com"}`)
	c, on, err := LoadConfig(dir)
	if err != nil || !on {
		t.Fatalf("on=%v err=%v", on, err)
	}
	want := Config{Host: "clalt.example.com", Team: "team-x", Aud: "aud-1", Email: "me@example.com"}
	if c != want {
		t.Fatalf("got %+v want %+v", c, want)
	}
}

func TestLoadConfigRefusesIncompleteOrUnsafeFiles(t *testing.T) {
	cases := map[string]string{
		"broken json":    `{`,
		"no host":        `{"team":"team-x","aud":"a","email":"me@example.com"}`,
		"no aud":         `{"host":"h.example.com","team":"team-x","email":"me@example.com"}`,
		"no email":       `{"host":"h.example.com","team":"team-x","aud":"a"}`,
		"no team":        `{"host":"h.example.com","aud":"a","email":"me@example.com"}`,
		"team with dots": `{"host":"h.example.com","team":"evil.com/x","aud":"a","email":"me@example.com"}`,
		"host with port": `{"host":"h.example.com:8080","team":"team-x","aud":"a","email":"me@example.com"}`,
		"loopback host":  `{"host":"localhost","team":"team-x","aud":"a","email":"me@example.com"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, on, err := LoadConfig(writeConfig(t, body)); err == nil || on {
				t.Fatalf("on=%v err=%v, want an error and off", on, err)
			}
		})
	}
}
