package main

import (
	"os"
	"strings"
	"testing"
)

// the installer prints to everyone who runs it, so it must not promise what
// was retired, and it says what to do next with the commands that exist
func TestTheInstallerPromisesNothingThatWasRetired(t *testing.T) {
	raw, err := os.ReadFile("scripts/install.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(raw))
	for _, gone := range []string{"keepalive", "vault", "add an account", "clalt add"} {
		if strings.Contains(text, gone) {
			t.Errorf("scripts/install.sh still mentions %q", gone)
		}
	}
	for _, next := range []string{"clalt help", "clalt login <name>"} {
		if !strings.Contains(text, next) {
			t.Errorf("scripts/install.sh does not tell what to do next: %q is missing", next)
		}
	}
}
