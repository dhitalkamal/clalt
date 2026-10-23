package terminal

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/dhitalkamal/clalt/internal/runcmd"
)

// FocusResult is what Focus did beyond succeeding.
type FocusResult struct {
	// Note says what did not work without being an error: tmux could not
	// switch a client, so the window and pane are selected but not shown.
	Note string
}

// CannotFocus is a session whose terminal clalt cannot bring to the front.
type CannotFocus struct {
	Kind Kind
	IDE  string
}

func (e *CannotFocus) Error() string {
	if e.Kind == IDE {
		return "a session in " + e.IDE + " cannot be brought to the front"
	}
	return "a session in a plain terminal cannot be brought to the front"
}

var tmuxPane = regexp.MustCompile(`^%[0-9]+$`)

// Focus brings the terminal at loc to the front.
func (d Driver) Focus(ctx context.Context, loc Location) (FocusResult, error) {
	switch loc.Kind {
	case Kitty:
		return FocusResult{}, d.focusKitty(ctx, loc)
	case Tmux:
		return d.focusTmux(ctx, loc)
	}
	return FocusResult{}, &CannotFocus{Kind: loc.Kind, IDE: loc.IDE}
}

func (d Driver) focusKitty(ctx context.Context, loc Location) error {
	if loc.KittyWindow <= 0 {
		return fmt.Errorf("the kitty window id %d is not valid", loc.KittyWindow)
	}
	if !strings.HasPrefix(loc.KittyListenOn, "unix:") && !strings.HasPrefix(loc.KittyListenOn, "tcp:") {
		return fmt.Errorf("the kitty address %q does not start with unix: or tcp:", loc.KittyListenOn)
	}
	if !d.Run.Has("kitten") {
		return fmt.Errorf("kitten is not on PATH")
	}
	_, err := d.Run.Run(ctx, "kitten", "@", "--to", loc.KittyListenOn, "focus-window", "--match", fmt.Sprintf("id:%d", loc.KittyWindow))
	if err != nil {
		return fmt.Errorf("kitten focus-window: %w", err)
	}
	return nil
}

func (d Driver) focusTmux(ctx context.Context, loc Location) (FocusResult, error) {
	if !tmuxPane.MatchString(loc.TmuxPane) {
		return FocusResult{}, fmt.Errorf("the tmux pane %q is not like %%5", loc.TmuxPane)
	}
	if !d.Run.Has("tmux") {
		return FocusResult{}, fmt.Errorf("tmux is not on PATH")
	}
	var prefix []string
	if loc.TmuxSocket != "" {
		prefix = []string{"-S", loc.TmuxSocket}
	}
	call := func(verb string) error {
		_, err := d.Run.Run(ctx, "tmux", append(append([]string(nil), prefix...), verb, "-t", loc.TmuxPane)...)
		return err
	}
	for _, verb := range []string{"select-window", "select-pane"} {
		if err := call(verb); err != nil {
			return FocusResult{}, fmt.Errorf("tmux %s: %w", verb, err)
		}
	}
	if err := call("switch-client"); err != nil {
		return FocusResult{Note: "the window is selected but tmux could not switch to it: " + failText(err)}, nil
	}
	return FocusResult{}, nil
}

// failText is the short reason of a failed call.
func failText(err error) string {
	if f, ok := err.(*runcmd.Failure); ok {
		if line := strings.TrimSpace(strings.SplitN(f.Stderr, "\n", 2)[0]); line != "" {
			return line
		}
	}
	return err.Error()
}
