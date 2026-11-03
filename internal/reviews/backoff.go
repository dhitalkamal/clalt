package reviews

import (
	"fmt"
	"strings"
	"time"
)

// maxAttempts is how many failed runs a review gets. the last failure is kept
// as a visible failed record and is never retried.
var maxAttempts = 5

// MaxAttempts is the attempt cap, for callers that show retry state.
func MaxAttempts() int {
	return maxAttempts
}

// giveUpText is appended to the error of the record that used the last attempt.
// built from maxAttempts so the cap is defined once.
func giveUpText() string {
	return fmt.Sprintf("gave up after %d attempts", maxAttempts)
}

// backoffSteps is the wait after failed attempt 1, 2, 3 and 4. later
// attempts wait as long as the last step.
var backoffSteps = []time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour, 3 * time.Hour}

// backoff is how long to wait after the nth failed attempt.
func backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > len(backoffSteps) {
		attempts = len(backoffSteps)
	}
	return backoffSteps[attempts-1]
}

// failedAttempts is how many failed runs a stored record stands for. a failed
// record from before attempts existed counts as one.
func failedAttempts(rv Review) int {
	if !isFailed(rv) {
		return 0
	}
	if rv.Attempts < 1 {
		return 1
	}
	return rv.Attempts
}

// retryDue says whether a failed record may be tried again at now. records
// without a next time (legacy) are due at once, unless out of attempts.
func retryDue(rv Review, now time.Time) bool {
	// out of attempts: never due, whatever the stored time says
	if failedAttempts(rv) >= maxAttempts {
		return false
	}
	return rv.NextRetryAt == nil || !now.Before(*rv.NextRetryAt)
}

// stampFailure sets Attempts and NextRetryAt on a freshly failed review, given
// the failed attempts that came before it. other results are left clean.
func stampFailure(rv *Review, prev int, now time.Time) {
	if !isFailed(*rv) {
		rv.Attempts, rv.NextRetryAt = 0, nil
		return
	}
	rv.Attempts = prev + 1
	if rv.Attempts >= maxAttempts {
		// the error was trimmed earlier, so the note is appended whole. the
		// contains check keeps an odd record from getting it twice.
		rv.NextRetryAt = nil
		if text := giveUpText(); !strings.Contains(rv.Error, text) {
			rv.Error += "; " + text
		}
		return
	}
	next := now.Add(backoff(rv.Attempts))
	rv.NextRetryAt = &next
}
