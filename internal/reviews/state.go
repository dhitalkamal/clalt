package reviews

import "context"

// prKey names one pull request across its stored reviews.
type prKey struct {
	repo   string
	number int
}

// RefreshStates re-checks the pull request state of stored reviews: one gh
// lookup per pull request, skipped once any of its reviews is merged or closed
// (those never reopen in practice). A good answer is written to every review
// of that pull request with the check time. A failed lookup changes nothing
// and is not logged, the next round retries it. It works whether or not
// reviews are enabled, so history stays current after the feature is turned off.
func (s *Service) RefreshStates(ctx context.Context) {
	s.tickMu.Lock()
	defer s.tickMu.Unlock()
	all, err := s.store().List()
	if err != nil {
		return
	}
	groups := map[prKey][]Review{}
	var order []prKey
	for _, rv := range all {
		k := prKey{rv.Repo, rv.Number}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], rv)
	}
	for _, k := range order {
		if ctx.Err() != nil {
			return
		}
		rvs := groups[k]
		if anyFinal(rvs) {
			continue
		}
		state, err := s.GH.PRState(ctx, k.repo, k.number)
		if err != nil {
			continue
		}
		checked := s.Now()
		for _, rv := range rvs {
			rv.State, rv.StateCheckedAt = state, &checked
			// a failed save leaves the old file, retried next round
			_ = s.store().Save(rv)
		}
	}
}

// anyFinal reports whether any review of a pull request is merged or closed.
func anyFinal(rvs []Review) bool {
	for _, rv := range rvs {
		if rv.State == StateMerged || rv.State == StateClosed {
			return true
		}
	}
	return false
}
