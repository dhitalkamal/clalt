package reviews

import (
	"context"
	"fmt"
	"strings"
)

// diffTooBig says whether gh failed because GitHub will not build the diff.
// the api answers HTTP 406 when a pull request is over its diff limits (more
// than 300 files is one of them).
func diffTooBig(err error) bool {
	return err != nil && strings.Contains(err.Error(), "HTTP 406")
}

// prDiff is the diff of the pull request checked out in wt. it asks GitHub
// first. when GitHub refuses a diff that big, it builds the same diff in the
// worktree instead: fetch the base commit, then diff HEAD against the merge
// base. any other gh error is returned as it is.
func (s *Service) prDiff(ctx context.Context, wt string, pr PR) (string, error) {
	diff, err := s.GH.Diff(ctx, pr.Repo, pr.Number)
	if !diffTooBig(err) {
		return diff, err
	}
	local, err := s.localDiff(ctx, wt, pr)
	if err != nil {
		return "", fmt.Errorf("reviews: GitHub refused the diff as too big, and the local diff failed: %w", err)
	}
	return local, nil
}

func (s *Service) localDiff(ctx context.Context, wt string, pr PR) (string, error) {
	base, err := s.GH.BaseSHA(ctx, pr.Repo, pr.Number)
	if err != nil {
		return "", err
	}
	if err := s.Git.FetchCommit(ctx, wt, base); err != nil {
		return "", err
	}
	return s.Git.DiffFromBase(ctx, wt, base)
}
