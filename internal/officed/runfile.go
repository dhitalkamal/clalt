// Package officed runs the office server in the background: the run file the
// server writes, starting and stopping it, and start at login.
package officed

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// DefaultPort is where the background office listens first, the next free one
// of PortRange follows when it is busy.
const (
	DefaultPort = 47321
	PortRange   = 20
	// MaxLog is the size at which office.log is moved to office.log.old
	MaxLog = 1 << 20
)

// ErrNoRun is no usable run file.
var ErrNoRun = errors.New("no run file")

// Run is the run file: the server that is (or was) running in the background.
// the token is in here and in the browser cookie only.
type Run struct {
	PID       int       `json:"pid"`
	Port      int       `json:"port"`
	Token     string    `json:"token"`
	StartedAt time.Time `json:"startedAt"`
	Version   string    `json:"version"`
}

// Link is the first-load URL, it carries the token.
func (r Run) Link() string { return fmt.Sprintf("http://127.0.0.1:%d/?t=%s", r.Port, r.Token) }

func runPath(dir string) string { return filepath.Join(dir, "run.json") }

// LogPath is the log of the background server.
func LogPath(dir string) string { return filepath.Join(dir, "office.log") }

// WriteRun writes the run file, 0600 in a 0700 dir, atomically.
func WriteRun(dir string, r Run) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(runPath(dir), raw, 0o600)
}

// ReadRun reads the run file. a missing, broken or empty one is ErrNoRun.
func ReadRun(dir string) (Run, error) {
	raw, err := fsutil.ReadFile(runPath(dir))
	if err != nil {
		return Run{}, ErrNoRun
	}
	var r Run
	if err := json.Unmarshal(raw, &r); err != nil || r.PID <= 0 || r.Port <= 0 || r.Token == "" {
		return Run{}, ErrNoRun
	}
	return r, nil
}

// RemoveRun removes the run file. with pid > 0 only when the file is that
// server's, so a new server's file is never removed by an old one.
func RemoveRun(dir string, pid int) {
	if pid > 0 {
		if r, err := ReadRun(dir); err != nil || r.PID != pid {
			return
		}
	}
	_ = os.Remove(runPath(dir))
}

// PickPort is the first port from start that free accepts, among count.
func PickPort(start, count int, free func(int) bool) (int, error) {
	for p := start; p < start+count; p++ {
		if free(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port from %d to %d", start, start+count-1)
}

// RotateLog moves a log over max bytes to path.old, replacing the old one.
func RotateLog(path string, max int64) error {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() <= max {
		return nil
	}
	return os.Rename(path, path+".old")
}

// LastErrorLine is the last line of the log's tail that looks like an error.
func LastErrorLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	const tail = 64 << 10
	buf := make([]byte, tail)
	n, _ := f.ReadAt(buf, max(0, fileSize(f)-tail))
	lines := bytes.Split(buf[:n], []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.ToLower(string(lines[i]))
		if strings.Contains(l, "error") || strings.Contains(l, "fail") || strings.Contains(l, "panic") {
			return strings.TrimSpace(string(lines[i]))
		}
	}
	return ""
}

func fileSize(f *os.File) int64 {
	fi, err := f.Stat()
	if err != nil {
		return 0
	}
	return fi.Size()
}
