package claudecli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Session builds the interactive claude session for configDir, with args passed
// through untouched. like Login it attaches no stdio and starts nothing: the
// caller owns the terminal. like the auth calls it refuses a folder claude must
// not be pointed at, see checkFolder, since a session writes into it for as
// long as it runs.
func Session(ctx context.Context, configDir string, args []string) (*exec.Cmd, error) {
	if err := checkFolder(configDir); err != nil {
		return nil, fmt.Errorf("claude session: %w", err)
	}
	env, _ := Env(os.Environ(), configDir)
	cmd, _, err := command(ctx, env, args...)
	return cmd, err
}
