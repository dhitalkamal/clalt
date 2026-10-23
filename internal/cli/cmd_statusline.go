package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// cmdStatusline is `clalt statusline`, which claude runs as the statusLine of a
// clalt run session. it records what the input says about the session and the
// account, and then is the user's own statusLine: it runs their command with the
// same input and prints its output unchanged, so the line looks as it did.
// with no handle in CLALT_SESSION it only passes through.
func (a *App) cmdStatusline(args []string) error {
	raw := readInput(a.In)
	if os.Getenv(statuslineChained) != "" {
		return nil
	}
	// recording runs beside the user's command, so the line is not slower
	recorded := make(chan struct{})
	go func() {
		defer close(recorded)
		defer func() {
			if r := recover(); r != nil {
				hookLog(a.clock(), "statusline", "panic: %v", r)
			}
		}()
		if h := os.Getenv(sessionEnvVar); registry.ValidHandle(h) {
			a.recordStatusline(h, raw)
		}
	}()
	defer func() { <-recorded }()
	user, ok := readUserStatusLine()
	if !ok {
		return nil
	}
	if code := runUserStatusLine(user, raw, a.Out, a.Err); code != 0 {
		return exitError{code}
	}
	return nil
}

type statusWindow struct {
	UsedPercentage *float64 `json:"used_percentage"`
	ResetsAt       *float64 `json:"resets_at"`
}

func (w *statusWindow) window() *feed.Window {
	if w == nil || w.UsedPercentage == nil || w.ResetsAt == nil {
		return nil
	}
	return &feed.Window{UsedPct: *w.UsedPercentage, ResetsAt: time.Unix(int64(*w.ResetsAt), 0).UTC()}
}

// statusInput is the part of claude's statusline input that clalt reads.
type statusInput struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Model     struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
	ContextWindow struct {
		UsedPercentage *float64 `json:"used_percentage"`
	} `json:"context_window"`
	RateLimits struct {
		FiveHour *statusWindow `json:"five_hour"`
		SevenDay *statusWindow `json:"seven_day"`
	} `json:"rate_limits"`
}

// recordStatusline writes what one statusline input says. the account comes
// from the session's record, and when that is gone from the account folder
// claude was started in, so the limits are kept even then.
func (a *App) recordStatusline(handle string, raw []byte) {
	now := a.clock()
	var in statusInput
	if err := json.Unmarshal(raw, &in); err != nil {
		hookLog(now, "statusline", "%s: the input is not json: %v", handle, err)
		return
	}
	reg := registry.Registry{Dir: config.SessionsDir(), LockWait: statuslineLockWait}
	store := feed.Store{Dir: config.FeedDir(), LockWait: statuslineLockWait}
	rec, found, _ := reg.Get(handle)
	account := rec.Account
	if !found {
		account = accountOfConfigDir(os.Getenv("CLAUDE_CONFIG_DIR"))
	}
	if account == "" {
		return
	}
	if err := store.MergeLimits(account, in.RateLimits.FiveHour.window(), in.RateLimits.SevenDay.window(), now); err != nil {
		hookLog(now, "statusline", "%s: record the limits: %v", handle, err)
	}
	if !found {
		return
	}
	model, cwd := in.Model.ID, in.Workspace.CurrentDir
	if model == "" {
		model = in.Model.DisplayName
	}
	if cwd == "" {
		cwd = in.Cwd
	}
	err := store.UpdateSession(handle, account, now, func(s *feed.SessionState) bool {
		if s.State == feed.StateEnded {
			return false
		}
		changed := false
		if model != "" && s.Model != model {
			s.Model, changed = model, true
		}
		if cwd != "" && s.Cwd != cwd {
			s.Cwd, changed = cwd, true
		}
		if p := in.ContextWindow.UsedPercentage; p != nil && (s.ContextPct == nil || *s.ContextPct != *p) {
			s.ContextPct, changed = p, true
		}
		return changed
	})
	if err != nil {
		hookLog(now, "statusline", "%s: update the feed: %v", handle, err)
	}
	if err := noteClaudeSession(reg, rec, in.SessionID, now); err != nil {
		hookLog(now, "statusline", "%s: record the claude session id: %v", handle, err)
	}
}

// accountOfConfigDir is the id of the account whose folder dir is, or "".
func accountOfConfigDir(dir string) string {
	if dir == "" {
		return ""
	}
	dir = filepath.Clean(dir)
	if filepath.Dir(dir) != filepath.Clean(config.AccountsDir()) {
		return ""
	}
	id := filepath.Base(dir)
	if config.CheckAccountID(id) != nil {
		return ""
	}
	return id
}
