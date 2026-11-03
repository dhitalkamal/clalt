package reviews

import (
	"strings"
	"time"
)

// sizeSkipMarker is part of the text older versions stored when a pull
// request was too big to review. those records are retried now.
const sizeSkipMarker = "over 1500 changed lines"

// retryable says whether a stored result should be reviewed again: an old
// size skip or a failed run whose pull request is still open or of unknown
// state. merged and closed ones stay as history. a failed run also waits for
// its backoff time; a size skip is not subject to backoff.
func retryable(rv Review, now time.Time) bool {
	if !isSizeSkip(rv) && !isFailed(rv) {
		return false
	}
	if isFailed(rv) && !isSizeSkip(rv) && !retryDue(rv, now) {
		return false
	}
	return rv.State == StateOpen || rv.State == StateUnknown || rv.State == ""
}

func isSizeSkip(rv Review) bool {
	return strings.Contains(rv.Skipped, sizeSkipMarker)
}

// isFailed is a run that errored out. partial reviews carry an error too but
// hold real findings, so they are results and never retried.
func isFailed(rv Review) bool {
	return rv.Error != "" && !strings.HasPrefix(rv.Error, partialPrefix)
}

// done says whether this commit already has a result that should stand.
func (s *Service) done(repo string, number int, sha string) bool {
	st := s.store()
	if !st.Has(repo, number, sha) {
		return false
	}
	rv, ok, err := st.Get(idOf(repo, number, sha))
	return err != nil || !ok || !retryable(rv, s.Now())
}
