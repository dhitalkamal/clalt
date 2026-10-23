package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

func writeHistory(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// one file is already in the shared folder, one differs there: nothing is left
// to copy and the check must not talk about copying
func TestOldProfilesWithOnlyConflictsLeftSaySoPlainly(t *testing.T) {
	e := newEnv(t)
	old := filepath.Join(config.ProfilesDir(), "work", "projects", "-p")
	shared := filepath.Join(config.ClaudeProjectsDir(), "-p")
	writeHistory(t, old, "same.jsonl", "one")
	writeHistory(t, shared, "same.jsonl", "one")
	writeHistory(t, old, "diff.jsonl", "mine")
	writeHistory(t, shared, "diff.jsonl", "theirs, and longer")

	_, rep := doctor(t, e)

	rep.want(t, "leftover", "oldProfiles", "warn", "all history is already in the shared folder")
	rep.want(t, "leftover", "oldProfiles", "warn", "1 file differs and stays only in the profile folder")
	rep.want(t, "leftover", "oldProfiles", "warn", "diff.jsonl")
	if c, _ := rep.find("leftover", "oldProfiles"); strings.Contains(c.Detail, "would copy") {
		t.Errorf("still talks about copying: %q", c.Detail)
	}
}

func TestOldProfilesWithFilesToCopyKeepTheCopyWording(t *testing.T) {
	e := newEnv(t)
	old := filepath.Join(config.ProfilesDir(), "work", "projects", "-p")
	writeHistory(t, old, "a.jsonl", "one")
	writeHistory(t, old, "b.jsonl", "two")
	shared := filepath.Join(config.ClaudeProjectsDir(), "-p")
	writeHistory(t, old, "diff.jsonl", "mine")
	writeHistory(t, shared, "diff.jsonl", "theirs, and longer")

	_, rep := doctor(t, e)

	rep.want(t, "leftover", "oldProfiles", "warn", "history not copied yet: would copy 2 files")
	rep.want(t, "leftover", "oldProfiles", "warn", "1 conflict")
}
