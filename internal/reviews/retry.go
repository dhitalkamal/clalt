package reviews

import "strings"

// sizeSkipMarker is part of the text older versions stored when a pull
// request was too big to review. those records are retried now.
const sizeSkipMarker = "over 1500 changed lines"

// retryable says whether a stored result should be reviewed again: an old
// size skip whose pull request is still open or of unknown state. merged and
// closed ones stay as history.
func retryable(rv Review) bool {
	if !strings.Contains(rv.Skipped, sizeSkipMarker) {
		return false
	}
	return rv.State == StateOpen || rv.State == StateUnknown
}

// done says whether this commit already has a result that should stand.
func (s *Service) done(repo string, number int, sha string) bool {
	st := s.store()
	if !st.Has(repo, number, sha) {
		return false
	}
	rv, ok, err := st.Get(idOf(repo, number, sha))
	return err != nil || !ok || !retryable(rv)
}
