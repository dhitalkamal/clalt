package account

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

// TestSwitchWritesBackBeforeLoading is the load-bearing invariant: the currently
// live credential (which may hold a freshly rotated refresh token) must be saved
// back to the outgoing account's slot BEFORE the target is loaded, or that account
// silently loses its rotated token and gets logged out.
func TestSwitchWritesBackBeforeLoading(t *testing.T) {
	s, _ := newStore(t)

	// A is active; its stored slot holds an OLD token
	_ = s.Save("A", mkCred("A-old", 100), Meta{})
	_ = s.Save("B", mkCred("B-tok", 200), Meta{})
	_ = s.SetActive("A")

	// simulate Claude Code having rotated A's token while it was live
	if err := s.live.Write(mkCred("A-rotated", 150)); err != nil {
		t.Fatal(err)
	}

	if err := s.Switch("B"); err != nil {
		t.Fatalf("Switch: %v", err)
	}

	// A's slot must now hold the rotated token, not the old one
	aCred, _, _ := s.Get("A")
	if aCred.AccessToken != "A-rotated" {
		t.Errorf("write-back failed: A slot = %q, want A-rotated", aCred.AccessToken)
	}
	// live store now holds B
	liveCred, _ := s.live.Read()
	if liveCred.AccessToken != "B-tok" {
		t.Errorf("target not loaded: live = %q, want B-tok", liveCred.AccessToken)
	}
	// active pointer moved
	if a, _ := s.Active(); a != "B" {
		t.Errorf("active = %q, want B", a)
	}
}

func TestSwitchMissingTargetIsNoOp(t *testing.T) {
	s, _ := newStore(t)
	_ = s.Save("A", mkCred("A-tok", 100), Meta{})
	_ = s.SetActive("A")
	_ = s.live.Write(mkCred("A-tok", 100))

	if err := s.Switch("ghost"); err == nil {
		t.Fatal("expected error switching to missing account")
	}
	// nothing changed
	if a, _ := s.Active(); a != "A" {
		t.Errorf("active moved to %q on failed switch", a)
	}
	liveCred, _ := s.live.Read()
	if liveCred.AccessToken != "A-tok" {
		t.Errorf("live changed on failed switch: %q", liveCred.AccessToken)
	}
}

func TestSwitchAppendsToLog(t *testing.T) {
	s, _ := newStore(t)
	_ = s.Save("A", mkCred("A", 1), Meta{})
	_ = s.Save("B", mkCred("B", 1), Meta{})
	_ = s.SetActive("A")
	_ = s.live.Write(mkCred("A", 1))
	_ = s.Switch("B")

	f, err := os.Open(s.logPath)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()
	var last map[string]any
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
		_ = json.Unmarshal(sc.Bytes(), &last)
	}
	if n == 0 {
		t.Fatal("no switch events logged")
	}
	if last["account"] != "B" {
		t.Errorf("last logged account = %v, want B", last["account"])
	}
}
