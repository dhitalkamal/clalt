package live

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/dashboard"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// Proc is what the system says about one process.
type Proc struct {
	Alive   bool
	Start   time.Time
	PPID    int
	Command string // the program and its arguments
}

// Stop ends a session by asking claude to quit. the dialog that asked may hold
// a handle that went stale, so everything is checked again here and anything
// that does not match is refused as a message: the handle must be in the
// registry under this account, its wrapper alive, and the pid it records must
// be a running claude that started after the session did and is a child of its
// wrapper. a recycled pid fails one of those. only claude gets SIGTERM, the
// wrapper then sees it exit and cleans up as it always does.
func (r *RegistrySessions) Stop(ctx context.Context, accountID, handle string) error {
	if feed.IsExt(handle) {
		return dashboard.Notice("That session is plain claude, not started by clalt run. clalt cannot stop it.")
	}
	rec, ok, err := r.Reg.Get(handle)
	switch {
	case err != nil:
		return fmt.Errorf("could not read the session record: %w", err)
	case !ok || rec.Account != accountID:
		return dashboard.Notice("That session is gone or is not on this account. Nothing was stopped.")
	case !rec.Alive():
		return dashboard.Notice("That session has already ended. Nothing was stopped.")
	case rec.WrapperPID <= 0:
		return dashboard.Notice("That session record is from an older clalt and cannot be checked. Nothing was stopped.")
	case rec.PID <= 0:
		return dashboard.Notice("claude has not started in that session yet. Nothing was stopped.")
	}
	probe, signal := r.Probe, r.Signal
	if probe == nil {
		probe = probeProc
	}
	if signal == nil {
		signal = signalPID
	}
	p, err := probe(ctx, rec.PID)
	if err != nil {
		return fmt.Errorf("could not check claude's process: %w", err)
	}
	if why := mismatch(rec, p); why != "" {
		return dashboard.Notice("Not stopped: " + why + ".")
	}
	if err := signal(rec.PID); err != nil {
		return fmt.Errorf("could not stop claude (pid %d): %w", rec.PID, err)
	}
	return nil
}

// startSlack is how far before the record a start may be and still count: ps
// prints seconds, and the two clocks may differ a little
const startSlack = time.Second

// mismatch says why p is not the claude of rec, or "" when it is. ps prints the
// start to the second, so the record's own start is cut to match.
func mismatch(rec registry.Record, p Proc) string {
	switch {
	case !p.Alive:
		return fmt.Sprintf("pid %d is not running", rec.PID)
	case p.Start.Before(rec.StartedAt.Truncate(time.Second).Add(-startSlack)):
		return fmt.Sprintf("pid %d started before the session did, so it is another process", rec.PID)
	case p.PPID != rec.WrapperPID:
		return fmt.Sprintf("pid %d does not belong to the session's clalt run", rec.PID)
	case !isClaudeCommand(p.Command):
		return fmt.Sprintf("pid %d is not claude", rec.PID)
	}
	return ""
}

// isClaudeCommand is true when the program, or the script a node or bun was
// given, is named claude. a word further along the line does not count.
func isClaudeCommand(command string) bool {
	exe, rest := splitCommand(command)
	if exe == "" {
		return false
	}
	switch base(exe) {
	case "claude", "claude.exe":
		return true
	case "node", "bun", "node.exe", "bun.exe":
		for _, a := range rest {
			if strings.HasPrefix(a, "-") {
				continue
			}
			return isClaudeScript(a)
		}
	}
	return false
}

func base(p string) string { return strings.ToLower(filepath.Base(strings.ReplaceAll(p, `\`, "/"))) }

// isClaudeScript is a script named claude, or claude's cli.js inside a package
// folder named for it
func isClaudeScript(path string) bool {
	switch base(path) {
	case "claude", "claude.exe":
		return true
	case "cli.js":
		dir := base(filepath.Dir(strings.ReplaceAll(path, `\`, "/")))
		return dir == "claude" || dir == "claude-code"
	}
	return false
}

// splitCommand finds the program in a command line that ps printed with the
// arguments joined by spaces. a path may hold spaces, so the longest start of
// the line that is a file wins; else the interpreter word, or everything
// before the first flag.
func splitCommand(command string) (exe string, rest []string) {
	f := strings.Fields(command)
	if len(f) == 0 {
		return "", nil
	}
	for n := len(f); n >= 1; n-- {
		cand := strings.Join(f[:n], " ")
		if fi, err := os.Stat(cand); err == nil && fi.Mode().IsRegular() {
			return cand, f[n:]
		}
	}
	switch base(f[0]) {
	case "node", "bun", "node.exe", "bun.exe":
		return f[0], f[1:]
	}
	n := 1
	for n < len(f) && !strings.HasPrefix(f[n], "-") {
		n++
	}
	return strings.Join(f[:n], " "), f[n:]
}

var errNoStop = errors.New("stopping sessions from the dashboard is not supported on this platform")
