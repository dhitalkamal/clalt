package reviews

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

// errMarker joins the kept head and tail of a long error text.
const errMarker = "\n...\n"

// noisyGitError is a multi-line failure: a first line, progress noise that
// pushes it well past 300 runes, and the real cause on the last line.
func noisyGitError() string {
	var b strings.Builder
	b.WriteString("Cloning into 'x'...\nremote: Enumerating objects: 100\nremote: Counting objects: 50%\n")
	for i := 0; i < 12; i++ {
		b.WriteString("remote: Compressing objects: 1234 of 5678 noise noise\n")
	}
	b.WriteString("fatal: unable to access 'https://github.com/o/r/': TLS handshake timeout")
	return b.String()
}

// wantHeadAndTail checks s starts with head, ends with tail, joins them with
// the marker and fits in limit runes.
func wantHeadAndTail(t *testing.T, s, head, tail string, limit int) {
	t.Helper()
	if !strings.HasPrefix(s, head) {
		t.Errorf("text = %q, want it to start with %q", s, head)
	}
	if !strings.HasSuffix(s, tail) {
		t.Errorf("text = %q, want it to end with %q", s, tail)
	}
	if !strings.Contains(s, errMarker) {
		t.Errorf("text = %q, want it to contain %q", s, errMarker)
	}
	if n := utf8.RuneCountInString(s); n > limit {
		t.Errorf("text has %d runes, want at most %d", n, limit)
	}
}

// failSmallRun makes every claude run of the small PR fail with text and
// returns the stored review.
func failSmallRun(t *testing.T, text string) Review {
	t.Helper()
	r := newRig(t)
	onlySmallPR(r)
	r.fx.fail = func(c call) error {
		if c.Name == "claude" {
			return errors.New(text)
		}
		return nil
	}
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatalf("tick = %v, want nil (the failure is saved)", err)
	}
	got, ok, err := r.svc.Get("o-r-7-1111111")
	if err != nil || !ok {
		t.Fatalf("get: %v %v", ok, err)
	}
	return got
}

func TestALongMultiLineRunFailureKeepsItsFirstLineAndItsLastLine(t *testing.T) {
	got := failSmallRun(t, noisyGitError())
	if got.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (no give-up note)", got.Attempts)
	}
	wantHeadAndTail(t, got.Error, "Cloning into", "TLS handshake timeout", 300)
}

func TestALongRunFailureWithoutNewlinesKeepsItsHeadAndItsTail(t *testing.T) {
	text := "HEADTEXT " + strings.Repeat("x", 600) + " TAILTEXT"
	got := failSmallRun(t, text)
	wantHeadAndTail(t, got.Error, "HEADTEXT", "TAILTEXT", 300)
}

func TestAShortRunFailureIsStoredUnchanged(t *testing.T) {
	text := "claude exited 1: limit reached\nsecond line"
	got := failSmallRun(t, text)
	if got.Error != text {
		t.Errorf("error = %q, want %q", got.Error, text)
	}
	if strings.Contains(got.Error, errMarker) {
		t.Errorf("error = %q, want no marker on a short text", got.Error)
	}
}

func TestALongChunkFailureReasonKeepsItsFirstLineAndItsLastLineInThePartialError(t *testing.T) {
	r := crossRig(t, sumReplies, envelope("sx", "x", ""))
	failChunks(r, map[int]string{2: noisyGitError()})
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok {
		t.Fatal("review not saved")
	}
	prefix := partialPrefix + "chunk 2 of 3 failed (files: b.go): "
	if !strings.HasPrefix(got.Error, prefix) {
		t.Fatalf("error = %q, want prefix %q", got.Error, prefix)
	}
	wantHeadAndTail(t, strings.TrimPrefix(got.Error, prefix), "Cloning into", "TLS handshake timeout", maxReason)
}

func TestEachOfTwoLongChunkFailureReasonsKeepsItsOwnTail(t *testing.T) {
	r := crossRig(t, sumReplies, envelope("sx", "x", ""))
	failChunks(r, map[int]string{
		1: strings.Replace(noisyGitError(), "TLS handshake timeout", "cause one", 1),
		3: strings.Replace(noisyGitError(), "TLS handshake timeout", "cause three", 1),
	})
	_ = r.svc.Tick(context.Background())
	got, _, _ := r.svc.Get("o-r-7-1111111")
	for _, w := range []string{"chunk 1 of 3 failed (files: a.go): ", "cause one; chunk 3 of 3 failed (files: c.go): ", "cause three"} {
		if !strings.Contains(got.Error, w) {
			t.Errorf("error = %q, want it to contain %q", got.Error, w)
		}
	}
}

func TestALongCrossFilePassFailureReasonKeepsItsFirstLineAndItsLastLine(t *testing.T) {
	r := crossRig(t, sumReplies, envelope("sx", "x", ""))
	r.fx.fail = func(c call) error {
		if c.Name == "claude" && isCrossCall(c) {
			return errors.New(noisyGitError())
		}
		return nil
	}
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok {
		t.Fatal("review not saved")
	}
	prefix := partialPrefix + "cross-file pass failed: "
	if !strings.HasPrefix(got.Error, prefix) {
		t.Fatalf("error = %q, want prefix %q", got.Error, prefix)
	}
	wantHeadAndTail(t, strings.TrimPrefix(got.Error, prefix), "Cloning into", "TLS handshake timeout", maxReason)
}
