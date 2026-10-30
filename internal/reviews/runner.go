package reviews

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// reviewTimeout bounds one claude run.
const reviewTimeout = 20 * time.Minute

// newSessionID is a random v4 uuid, the id the claude run is given.
func newSessionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// claudeArgs is the whole argv of a review run. the tools are the three read
// ones and nothing else, anything that would still ask is denied, project
// settings (hooks) and MCP servers of the pull request are not loaded, and the
// prompt comes on stdin.
func claudeArgs(sessionID string) []string {
	return []string{
		"-p", "--output-format", "json", "--session-id", sessionID,
		"--tools", "Read,Grep,Glob", "--allowedTools", "Read,Grep,Glob",
		"--disallowedTools", "Bash,Edit,Write,NotebookEdit,WebFetch,WebSearch",
		"--permission-mode", "dontAsk", "--setting-sources", "user", "--strict-mcp-config",
	}
}

// clonePath is where the clone of repo lives.
func (s *Service) clonePath(repo string) string {
	owner, name, _ := strings.Cut(repo, "/")
	return filepath.Join(s.CacheDir, owner, name)
}

// ensureClone makes the clone once.
func (s *Service) ensureClone(ctx context.Context, repo string) (string, error) {
	path := s.clonePath(repo)
	if fi, err := os.Stat(filepath.Join(path, ".git")); err == nil && fi != nil {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := s.GH.Clone(ctx, repo, path); err != nil {
		return "", err
	}
	return path, nil
}

// review runs one review in a temporary worktree and returns the result. cand
// is the account of the first claude run. a failure is in the result's Error:
// it is stored like any other result so the page can show it. the only error
// returned is errNoRoom (a later run found no account with room): the review
// is then dropped, not stored.
func (s *Service) review(ctx context.Context, pr PR, info PRInfo, cand Candidate) (Review, error) {
	rv := Review{
		Repo: pr.Repo, Number: pr.Number, Title: tidy(info.Title, maxTitle), Author: tidy(info.Author, 80), URL: pr.URL, SHA: info.SHA,
		StartedAt: s.Now(), Account: cand.ID, Findings: []Finding{}, Transcript: newSessionID(),
	}
	if rv.Title == "" {
		rv.Title = tidy(pr.Title, maxTitle)
	}
	if rv.Author == "" {
		rv.Author = tidy(pr.Author, 80)
	}
	fail := func(err error) (Review, error) {
		rv.Error = tidy(err.Error(), 300)
		rv.EndedAt = s.Now()
		return rv, nil
	}
	clone, err := s.ensureClone(ctx, pr.Repo)
	if err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(s.WorkDir, 0o700); err != nil {
		return fail(err)
	}
	parent, err := os.MkdirTemp(s.WorkDir, "wt-")
	if err != nil {
		return fail(err)
	}
	wt := filepath.Join(parent, "wt")
	defer func() {
		// cleanup must happen even when the run was cancelled
		cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if s.Git.RemoveWorktree(cctx, clone, wt) != nil {
			_ = s.Git.Prune(cctx, clone)
		}
		_ = os.RemoveAll(parent)
	}()
	if err := s.Git.AddWorktree(ctx, clone, wt); err != nil {
		return fail(err)
	}
	if err := s.GH.Checkout(ctx, wt, pr.Repo, pr.Number); err != nil {
		return fail(err)
	}
	diff, err := s.GH.Diff(ctx, pr.Repo, pr.Number)
	if err != nil {
		return fail(err)
	}
	budget := s.ChunkBytes
	if budget <= 0 {
		budget = chunkBytes
	}
	res, err := s.reviewChunks(ctx, wt, pr, info, cand, splitDiff(diff, budget), diff, rv.Transcript)
	if errors.Is(err, errNoRoom) {
		return Review{}, err
	}
	if err != nil {
		return fail(err)
	}
	rv.Findings, rv.Summary, rv.Transcript, rv.Error = res.Findings, res.Summary, res.Transcript, res.Error
	rv.EndedAt = s.Now()
	return rv, nil
}
