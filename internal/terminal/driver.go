package terminal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dhitalkamal/clalt/internal/runcmd"
)

// ErrNoTerminal means neither tmux nor kitty is reachable, so the caller has to
// show the command instead.
var ErrNoTerminal = errors.New("no kitty socket or tmux here")

// Driver opens and focuses terminals through programs that are already running.
type Driver struct {
	Run runcmd.Runner
	Env func(name string) string
}

// LaunchSpec is the session to start in a new tab.
type LaunchSpec struct {
	Exe     string
	Account string
	Cwd     string
	Args    []string
}

// passedOn are the only variables a new tab gets from the caller. the tab starts
// with the environment of tmux or kitty, not ours.
var passedOn = []string{"PATH", "XDG_CONFIG_HOME", "CLAUDE_CONFIG_DIR", "CLALT_CLAUDE_BIN"}

// command is the argv the new tab runs: no shell is involved.
func (d Driver) command(spec LaunchSpec) []string {
	var env []string
	for _, name := range passedOn {
		if v := d.Env(name); v != "" {
			env = append(env, name+"="+v)
		}
	}
	var argv []string
	if len(env) > 0 {
		argv = append(append(argv, "env"), env...)
	}
	argv = append(argv, spec.Exe, "run", spec.Account)
	if len(spec.Args) > 0 {
		argv = append(append(argv, "--"), spec.Args...)
	}
	return argv
}

// Launch opens spec in a new tmux window or kitty tab and says where.
func (d Driver) Launch(ctx context.Context, spec LaunchSpec) (string, error) {
	cmd := d.command(spec)
	switch {
	case d.Env("TMUX") != "" && d.Run.Has("tmux"):
		args := append([]string{"new-window", "-P", "-c", spec.Cwd}, cmd...)
		out, err := d.Run.Run(ctx, "tmux", args...)
		if err != nil {
			return "", fmt.Errorf("tmux new-window: %w", err)
		}
		return "tmux window " + strings.TrimSpace(string(out)), nil
	case d.Env("KITTY_LISTEN_ON") != "" && d.Run.Has("kitten"):
		args := append([]string{"@", "--to", d.Env("KITTY_LISTEN_ON"), "launch", "--type=tab", "--cwd=" + spec.Cwd}, cmd...)
		out, err := d.Run.Run(ctx, "kitten", args...)
		if err != nil {
			return "", fmt.Errorf("kitten launch: %w", err)
		}
		return "kitty window " + strings.TrimSpace(string(out)), nil
	}
	return "", ErrNoTerminal
}
