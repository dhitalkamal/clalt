package reviews

import (
	"strings"
	"testing"
	"time"
)

// setCap overrides the attempt cap for one test and restores it after.
func setCap(t *testing.T, n int) {
	t.Helper()
	old := maxAttempts
	maxAttempts = n
	t.Cleanup(func() { maxAttempts = old })
}

func TestGiveUpMessageNamesTheOverriddenCapExactlyOnce(t *testing.T) {
	setCap(t, 3)
	rv := Review{Error: "claude exited 1: limit reached"}
	stampFailure(&rv, maxAttempts-1, time.Unix(5000, 0).UTC())
	if rv.NextRetryAt != nil {
		t.Fatalf("nextRetryAt = %v, want nil at the cap", rv.NextRetryAt)
	}
	const want = "gave up after 3 attempts"
	if n := strings.Count(rv.Error, want); n != 1 {
		t.Fatalf("error = %q, want %q exactly once, got %d", rv.Error, want, n)
	}
	if strings.Contains(rv.Error, "5") {
		t.Fatalf("error = %q, want no hand-written 5", rv.Error)
	}
}

func TestGiveUpMessageIsNotAddedBeforeTheOverriddenCap(t *testing.T) {
	setCap(t, 3)
	rv := Review{Error: "claude exited 1: limit reached"}
	stampFailure(&rv, maxAttempts-2, time.Unix(5000, 0).UTC())
	if strings.Contains(rv.Error, "gave up") {
		t.Fatalf("error = %q, want no give-up note below the cap", rv.Error)
	}
}
