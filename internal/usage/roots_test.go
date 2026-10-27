package usage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCollectRootsDedupsSymlinkedProfile proves a profile whose projects dir is
// a symlink back to the shared Claude projects dir is not scanned twice.
func TestCollectRootsDedupsSymlinkedProfile(t *testing.T) {
	base := t.TempDir()
	shared := filepath.Join(base, "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(base, "profile-a-projects")
	if err := os.Symlink(shared, linked); err != nil {
		t.Fatal(err)
	}

	got := CollectRoots([]string{shared, linked})
	if len(got) != 1 {
		t.Fatalf("got %d roots, want 1 (symlinked dup dropped): %v", len(got), got)
	}
}

// TestCollectRootsKeepsRealProfileDir proves a profile with its OWN real
// (non-symlinked) projects dir is kept as a distinct root, not dropped.
func TestCollectRootsKeepsRealProfileDir(t *testing.T) {
	base := t.TempDir()
	shared := filepath.Join(base, "shared")
	real := filepath.Join(base, "profile-real-projects")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}

	got := CollectRoots([]string{shared, real})
	if len(got) != 2 {
		t.Fatalf("got %d roots, want 2 (distinct real dirs kept): %v", len(got), got)
	}
}

// TestCollectRootsToleratesMissingDir proves a profile with no projects dir at
// all (never launched a session) does not break root collection.
func TestCollectRootsToleratesMissingDir(t *testing.T) {
	base := t.TempDir()
	shared := filepath.Join(base, "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(base, "never-existed")

	got := CollectRoots([]string{shared, missing})
	if len(got) != 2 {
		t.Fatalf("got %d roots, want 2 (missing dir kept, just yields no records): %v", len(got), got)
	}
}

// TestScanRootsDedupsAcrossRoots proves the same (message.id, requestId)
// appearing in two different roots (e.g. a resumed session visible under both
// the shared dir and a profile dir) collapses to one record.
func TestScanRootsDedupsAcrossRoots(t *testing.T) {
	base := t.TempDir()
	rootA := filepath.Join(base, "a", "proj")
	rootB := filepath.Join(base, "b", "proj")
	if err := os.MkdirAll(rootA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rootB, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"type":"assistant","timestamp":"2026-08-24T10:00:00.000Z","sessionId":"s1","requestId":"req_1","message":{"id":"msg_1","model":"claude-sonnet-5","usage":{"input_tokens":10,"output_tokens":10}}}` + "\n"
	if err := os.WriteFile(filepath.Join(rootA, "s.jsonl"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootB, "s.jsonl"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	recs, err := ScanRoots([]string{filepath.Join(base, "a"), filepath.Join(base, "b")})
	if err != nil {
		t.Fatalf("ScanRoots: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1 (deduped across roots): %+v", len(recs), recs)
	}
}
