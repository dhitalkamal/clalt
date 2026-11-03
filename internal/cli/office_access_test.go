package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficeAccessIsOffWithoutAConfig(t *testing.T) {
	var logged []string
	if v := officeAccess(t.TempDir(), func(f string, a ...any) { logged = append(logged, f) }); v != nil {
		t.Fatal("access on with no access.json")
	}
	if len(logged) != 0 {
		t.Fatalf("logged %v for a missing file", logged)
	}
}

func TestOfficeAccessLoadsAGoodConfig(t *testing.T) {
	dir := t.TempDir()
	body := `{"host":"clalt.example.com","team":"team-x","aud":"a","email":"me@example.com"}`
	if err := os.WriteFile(filepath.Join(dir, "access.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	v := officeAccess(dir, func(string, ...any) {})
	if v == nil || v.Config().Host != "clalt.example.com" {
		t.Fatalf("got %v, want a verifier for clalt.example.com", v)
	}
}

// a broken file must leave access off and say so, never half on
func TestOfficeAccessLogsAndStaysOffForABrokenConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "access.json"), []byte(`{"host":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var logged []string
	v := officeAccess(dir, func(f string, a ...any) { logged = append(logged, f) })
	if v != nil {
		t.Fatal("access on with a broken config")
	}
	if len(logged) != 1 || !strings.Contains(logged[0], "access") {
		t.Fatalf("logged %v, want one access line", logged)
	}
}
