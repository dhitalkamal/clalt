package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feedinstall"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// statuslineWait is how long the user's own statusLine command gets. claude
// shows nothing for a run that is not done by its next update anyway.
var statuslineWait = 3 * time.Second

// statuslineChained is set for the user's command. a statusLine that is itself
// `clalt statusline` would run itself forever otherwise.
const statuslineChained = "CLALT_STATUSLINE_CHAINED"

// userStatusLine is the statusLine of the user's own settings: the command, and
// the optional keys that change how claude draws it.
type userStatusLine struct {
	Command string
	Extras  map[string]json.RawMessage // padding, refreshInterval, hideVimModeIndicator
}

// readUserStatusLine reads statusLine from settings.json of the shared claude
// home, and only from there. a project's own settings are never read: their
// commands wait for the workspace trust dialog, and clalt would run one without.
func readUserStatusLine() (userStatusLine, bool) {
	dir := config.SharedClaudeDir()
	if dir == "" {
		return userStatusLine{}, false
	}
	raw, err := fsutil.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		return userStatusLine{}, false
	}
	var s struct {
		StatusLine map[string]json.RawMessage `json:"statusLine"`
	}
	if json.Unmarshal(raw, &s) != nil || s.StatusLine == nil {
		return userStatusLine{}, false
	}
	u, ok := parseUserStatusLine(s.StatusLine)
	if ok && feedinstall.IsOurs(u.Command) {
		// clalt feed install put ours here, and kept the user's in its own state
		orig, found := feedinstall.LoadOriginal(feedInstallPath())
		var o map[string]json.RawMessage
		if !found || json.Unmarshal(orig, &o) != nil {
			return userStatusLine{}, false
		}
		return parseUserStatusLine(o)
	}
	return u, ok
}

// parseUserStatusLine reads a statusLine object: a command, and the optional
// keys that change how claude draws it.
func parseUserStatusLine(line map[string]json.RawMessage) (userStatusLine, bool) {
	var kind, command string
	_ = json.Unmarshal(line["type"], &kind)
	_ = json.Unmarshal(line["command"], &command)
	if kind != "command" || strings.TrimSpace(command) == "" {
		return userStatusLine{}, false
	}
	u := userStatusLine{Command: command, Extras: map[string]json.RawMessage{}}
	for _, k := range []string{"padding", "refreshInterval", "hideVimModeIndicator"} {
		if v, ok := line[k]; ok {
			u.Extras[k] = v
		}
	}
	return u, true
}

// runUserStatusLine runs the user's command with the same input, passing its
// output and stderr through, and returns the exit code claude should see.
func runUserStatusLine(u userStatusLine, stdin []byte, out, errOut io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), statuslineWait)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", u.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", u.Command)
	}
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout, cmd.Stderr = out, errOut
	cmd.Env = append(os.Environ(), statuslineChained+"=1")
	// a killed shell can leave a child that holds the pipes
	cmd.WaitDelay = 200 * time.Millisecond
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit) && exit.ExitCode() > 0 && ctx.Err() == nil:
		return exit.ExitCode()
	}
	return 1
}
