package reviews

import (
	"context"
	"fmt"
	"strings"
)

// Git is the one way this package runs git, and only for the local worktree
// of a review: add one, remove one, prune. nothing here talks to a remote.
type Git struct{ X Exec }

// allowedGit says whether argv is one of the three worktree commands.
func allowedGit(args []string) error {
	ok := false
	switch {
	case len(args) == 4 && args[0] == "worktree" && args[1] == "add" && args[2] == "--detach":
		ok = !strings.HasPrefix(args[3], "-")
	case len(args) == 4 && args[0] == "worktree" && args[1] == "remove" && args[2] == "--force":
		ok = !strings.HasPrefix(args[3], "-")
	case len(args) == 2 && args[0] == "worktree" && args[1] == "prune":
		ok = true
	}
	if !ok {
		return fmt.Errorf("reviews: this git command is not allowed: git %s", strings.Join(args, " "))
	}
	return nil
}

func (g Git) run(ctx context.Context, clone string, args ...string) error {
	if err := allowedGit(args); err != nil {
		return err
	}
	_, err := g.X.Run(ctx, clone, nil, "", "git", args...)
	return err
}

// AddWorktree makes a detached worktree of the clone at path.
func (g Git) AddWorktree(ctx context.Context, clone, path string) error {
	return g.run(ctx, clone, "worktree", "add", "--detach", path)
}

// RemoveWorktree deletes the worktree at path from the clone.
func (g Git) RemoveWorktree(ctx context.Context, clone, path string) error {
	return g.run(ctx, clone, "worktree", "remove", "--force", path)
}

// Prune forgets worktrees whose folder is gone.
func (g Git) Prune(ctx context.Context, clone string) error {
	return g.run(ctx, clone, "worktree", "prune")
}
