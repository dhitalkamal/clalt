package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Session records a live `clalt run` process so the dashboard can list parallel
// sessions. Records are keyed by PID and removed when the process exits.
type Session struct {
	Account   string    `json:"account"`
	PID       int       `json:"pid"`
	Cwd       string    `json:"cwd"`
	StartedAt time.Time `json:"startedAt"`
}

func sessionPath(dir string, pid int) string {
	return filepath.Join(dir, strconv.Itoa(pid)+".json")
}

// WriteSession records a running session.
func WriteSession(dir string, s Session) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(sessionPath(dir, s.PID), raw, 0o600)
}

// RemoveSession clears a session record (missing is not an error).
func RemoveSession(dir string, pid int) error {
	err := os.Remove(sessionPath(dir, pid))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ListSessions returns live sessions, pruning records whose process has exited.
func ListSessions(dir string) ([]Session, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			continue
		}
		var s Session
		if json.Unmarshal(raw, &s) != nil {
			continue
		}
		if !alive(s.PID) {
			_ = os.Remove(filepath.Join(dir, e.Name())) // prune dead
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out, nil
}

// alive reports whether a process with pid exists. It is implemented per-OS in
// alive_unix.go and alive_windows.go because the liveness check differs.
