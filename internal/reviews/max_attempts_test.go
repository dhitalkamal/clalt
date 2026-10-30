package reviews

import "testing"

func TestMaxAttemptsFollowsTheOverriddenCap(t *testing.T) {
	setCap(t, 3)
	if got := MaxAttempts(); got != 3 {
		t.Fatalf("MaxAttempts() = %d, want 3", got)
	}
}

func TestMaxAttemptsDefaultEqualsTheCapVar(t *testing.T) {
	if got := MaxAttempts(); got != maxAttempts {
		t.Fatalf("MaxAttempts() = %d, want %d", got, maxAttempts)
	}
}
