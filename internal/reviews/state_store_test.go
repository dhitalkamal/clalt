package reviews

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// oldFormat is a review file written before state existed: no state fields.
const oldFormat = `{
  "repo": "o/r",
  "number": 7,
  "title": "Fix it",
  "author": "ann",
  "url": "https://x/7",
  "sha": "aaaaaaa11",
  "startedAt": "1970-01-01T00:16:47Z",
  "endedAt": "1970-01-01T00:33:27Z",
  "account": "work",
  "findings": [{"severity": "high", "file": "a.go", "line": 3, "title": "t", "detail": "d"}],
  "summary": "s",
  "transcript": "abc-123"
}`

func TestStoreKeepsStateAndCheckedAtThroughGetAndList(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	r := sample(1, "aaaaaaaa11")
	r.State = "merged"
	r.StateCheckedAt = &at
	if err := s.Save(r); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.Get(r.ID())
	if err != nil || !ok {
		t.Fatalf("get = %v %v", ok, err)
	}
	if got.State != "merged" || got.StateCheckedAt == nil || !got.StateCheckedAt.Equal(at) {
		t.Fatalf("get state = %q %v, want merged %v", got.State, got.StateCheckedAt, at)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v %v", list, err)
	}
	if list[0].State != "merged" || list[0].StateCheckedAt == nil || !list[0].StateCheckedAt.Equal(at) {
		t.Fatalf("list state = %q %v, want merged %v", list[0].State, list[0].StateCheckedAt, at)
	}
}

func TestStoreReadsOldFormatFileAsUnknownState(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "o-r-7-aaaaaaa.json"), []byte(oldFormat), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Store{Dir: dir}
	got, ok, err := s.Get("o-r-7-aaaaaaa")
	if err != nil || !ok {
		t.Fatalf("get = %v %v", ok, err)
	}
	if got.State != "unknown" || got.StateCheckedAt != nil {
		t.Fatalf("get state = %q %v, want unknown nil", got.State, got.StateCheckedAt)
	}
	if got.Number != 7 || got.Summary != "s" || len(got.Findings) != 1 {
		t.Fatalf("old fields lost: %+v", got)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v %v", list, err)
	}
	if list[0].State != "unknown" || list[0].StateCheckedAt != nil {
		t.Fatalf("list state = %q %v, want unknown nil", list[0].State, list[0].StateCheckedAt)
	}
}

func TestStoreReadsEmptySavedStateAsUnknown(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	r := sample(2, "bbbbbbbb22")
	if err := s.Save(r); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.Get(r.ID())
	if err != nil || !ok {
		t.Fatalf("get = %v %v", ok, err)
	}
	if got.State != "unknown" || got.StateCheckedAt != nil {
		t.Fatalf("state = %q %v, want unknown nil", got.State, got.StateCheckedAt)
	}
}
