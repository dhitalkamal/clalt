package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dhitalkamal/clalt/internal/terminal"
)

const jumpUsage = "clalt jump <session>"

// cmdJump brings the terminal of a running session to the front. kitty and tmux
// are focused; for an IDE or a plain terminal there is nothing clalt can raise,
// so it says where the session runs.
func (a *App) cmdJump(args []string) error {
	if len(args) == 0 {
		return usageError{"which session? give a handle or the start of a session id", jumpUsage}
	}
	if strings.HasPrefix(args[0], "-") {
		return usageError{"unknown flag " + args[0], jumpUsage}
	}
	if len(args) > 1 {
		return usageError{"unexpected argument " + args[1], jumpUsage}
	}
	d := a.deps()
	running, err := d.sessions()
	if err != nil {
		return fmt.Errorf("could not read the running sessions: %w", err)
	}
	hits := matchSessions(running, args[0])
	if len(hits) == 0 {
		return fmt.Errorf("no running session matches %q", clean(args[0]))
	}
	labels, err := a.labelsByID()
	if err != nil {
		return err
	}
	labelOf := func(s terminal.Session) string {
		if l, ok := labels[s.Account]; ok {
			return l
		}
		return clean(s.Account)
	}
	if len(hits) > 1 {
		for _, s := range hits {
			fmt.Fprintf(a.Err, "  %s  %s  %s\n", clean(s.Handle), labelOf(s), clean(s.Cwd))
		}
		return fmt.Errorf("%d sessions match %q. use more of the handle", len(hits), clean(args[0]))
	}

	s := hits[0]
	drv := terminal.Driver{Run: d.runner, Env: d.env}
	res, err := drv.Focus(context.Background(), s.Location)
	var cannot *terminal.CannotFocus
	switch {
	case errors.As(err, &cannot):
		where := "a plain terminal"
		if cannot.Kind == terminal.IDE {
			where = clean(cannot.IDE)
		}
		a.printf("%s runs in %s, in %s, as %s. clalt cannot bring it to the front.\n",
			clean(s.Handle), where, clean(s.Cwd), labelOf(s))
		return nil
	case err != nil:
		return cleanErr(err)
	}
	a.printf("brought %s (%s) to the front\n", clean(s.Handle), labelOf(s))
	if res.Note != "" {
		a.printf("note: %s\n", clean(res.Note))
	}
	return nil
}

// matchSessions is the session with exactly this handle, else every session
// whose handle or claude session id starts with it.
func matchSessions(running []terminal.Session, want string) []terminal.Session {
	for _, s := range running {
		if s.Handle == want {
			return []terminal.Session{s}
		}
	}
	var hits []terminal.Session
	for _, s := range running {
		if strings.HasPrefix(s.Handle, want) || (s.ClaudeSessionID != "" && strings.HasPrefix(s.ClaudeSessionID, want)) {
			hits = append(hits, s)
		}
	}
	return hits
}
