package dashboard

import (
	"strings"
	"testing"
)

// Render draws what a source returned for a pipe, and what a source returns can
// hold anything: the program cleans every text of a snapshot, so Render must too
func TestRenderCleansTheSnapshotLikeTheProgramDoes(t *testing.T) {
	snap := fiveSnapshot()
	snap.Accounts[0].Label = "evil\x07\nforged line"
	snap.Accounts[1].Email = "p\x1b]0;owned\x07@example.com"

	out := Render(snap, 100, 30, testOptions())

	if strings.ContainsAny(out, "\a\r\x1b") {
		t.Errorf("a control character reached the frame: %q", out)
	}
	if n := strings.Count(out, "\n"); n != 29 {
		t.Errorf("30 lines have 29 newlines, got %d: a label drew its own lines\n%s", n, out)
	}
	if !strings.Contains(out, "evil") {
		t.Errorf("the label is still shown, cleaned:\n%s", out)
	}
}
