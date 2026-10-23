package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
)

const (
	// the hook reads at most this much of its input. payloads of tool results
	// can be megabytes, and none of the fields clalt reads is in one.
	hookInputMax = 1 << 20
	// a claude that never closes the pipe must not hold the hook up
	hookReadWait = time.Second
	// a hook waits this long for the lock of a folder. the lock is held for
	// microseconds, so this only runs out on a stuck process, and then a skipped
	// update costs less than a stalled claude. SessionEnd gets less, since claude
	// gives those hooks 1.5 seconds in all
	hookLockWait       = 1500 * time.Millisecond
	sessionEndLockWait = 700 * time.Millisecond
	// the statusline runs after every reply and nobody waits for what it records
	statuslineLockWait = 300 * time.Millisecond

	hookLogName = "hook.log"
	hookLogMax  = 256 << 10
)

// readInput reads what claude wrote to a hook or statusline command, up to
// hookInputMax, and drains the rest so that claude's write never meets a closed
// pipe. it returns after hookReadWait whatever happened.
func readInput(r io.Reader) []byte {
	data := make(chan []byte, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		b, _ := io.ReadAll(io.LimitReader(r, hookInputMax))
		data <- b
		_, _ = io.Copy(io.Discard, r)
	}()
	timer := time.NewTimer(hookReadWait)
	defer timer.Stop()
	var out []byte
	select {
	case out = <-data:
	case <-timer.C:
		return nil
	}
	select {
	case <-done:
	case <-timer.C:
	}
	return out
}

// hookLog appends one line to the hook log. it is where hook and statusline
// errors go, since their stdout feeds claude and their stderr is lost. a log
// that cannot be written is given up on: nothing here may fail a hook.
func hookLog(now time.Time, scope, format string, args ...any) {
	dir := config.LogsDir()
	if dir == "" || os.MkdirAll(dir, 0o700) != nil {
		return
	}
	path := filepath.Join(dir, hookLogName)
	if fi, err := os.Stat(path); err == nil && fi.Size() > hookLogMax {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	msg := strings.Join(strings.Fields(clean(fmt.Sprintf(format, args...))), " ")
	fmt.Fprintf(f, "%s %s %s\n", now.UTC().Format(time.RFC3339), scope, msg)
}
