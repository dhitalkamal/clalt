package reviews

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
)

// Git is the one way this package runs git, and only for the local worktree
// of a review: add one, remove one, prune, and the local diff for a pull
// request too big for GitHub to diff. the only remote call is a fetch of one
// commit from origin, which reads and never writes.
type Git struct{ X Exec }

// allowedGit says whether argv is one of the allowed commands, word for word.
// the only free words are a path that is not a flag and a full commit id.
func allowedGit(args []string) error {
	ok := false
	switch {
	case len(args) == 4 && args[0] == "worktree" && args[1] == "add" && args[2] == "--detach":
		ok = !strings.HasPrefix(args[3], "-")
	case len(args) == 4 && args[0] == "worktree" && args[1] == "remove" && args[2] == "--force":
		ok = !strings.HasPrefix(args[3], "-")
	case len(args) == 2 && args[0] == "worktree" && args[1] == "prune":
		ok = true
	case len(args) == 4 && args[0] == "fetch":
		ok = slices.Equal(args[:3], []string{"fetch", "--no-tags", "origin"}) && shaRe.MatchString(args[3])
	case len(args) == 7 && args[0] == "diff-tree":
		ok = slices.Equal(args[:5], []string{"diff-tree", "-p", "-r", "-M", "--merge-base"}) &&
			shaRe.MatchString(args[5]) && args[6] == "HEAD"
	}
	if !ok {
		return fmt.Errorf("reviews: this git command is not allowed: git %s", strings.Join(args, " "))
	}
	return nil
}

// run runs git in dir. env nil inherits the environment.
func (g Git) run(ctx context.Context, dir string, env []string, args ...string) ([]byte, error) {
	if err := allowedGit(args); err != nil {
		return nil, err
	}
	return g.X.Run(ctx, dir, env, "", "git", args...)
}

// AddWorktree makes a detached worktree of the clone at path.
func (g Git) AddWorktree(ctx context.Context, clone, path string) error {
	_, err := g.run(ctx, clone, nil, "worktree", "add", "--detach", path)
	return err
}

// RemoveWorktree deletes the worktree at path from the clone.
func (g Git) RemoveWorktree(ctx context.Context, clone, path string) error {
	_, err := g.run(ctx, clone, nil, "worktree", "remove", "--force", path)
	return err
}

// Prune forgets worktrees whose folder is gone.
func (g Git) Prune(ctx context.Context, clone string) error {
	_, err := g.run(ctx, clone, nil, "worktree", "prune")
	return err
}

// FetchCommit brings one commit of origin into the clone, run from the
// worktree at dir so FETCH_HEAD stays private to it (two reviews of one repo
// can fetch at once). a missing login fails instead of waiting at a prompt
// nobody sees.
func (g Git) FetchCommit(ctx context.Context, dir, sha string) error {
	_, err := g.run(ctx, dir, append(os.Environ(), "GIT_TERMINAL_PROMPT=0"), "fetch", "--no-tags", "origin", sha)
	return err
}

// DiffFromBase is the diff of HEAD in dir against its merge base with base,
// the same diff GitHub shows for a pull request. diff-tree is plumbing, so the
// user's diff config (no prefix, color, an external diff tool, textconv) does
// not apply and the headers are always "diff --git a/x b/y".
func (g Git) DiffFromBase(ctx context.Context, dir, base string) (string, error) {
	out, err := g.run(ctx, dir, nil, "diff-tree", "-p", "-r", "-M", "--merge-base", base, "HEAD")
	return string(out), err
}
