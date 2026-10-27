package wrapper

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// stopPath is where a stop request for the session is kept: <handle>.stop next
// to the record, like the move request. only the wrapper of the session reads
// it, so nothing but a session started by clalt run can ever be stopped.
func stopPath(dir, handle string) (string, error) {
	p, err := RequestPath(dir, handle)
	if err != nil {
		return "", err
	}
	return p[:len(p)-len(".move")] + ".stop", nil
}

// WriteStop asks the wrapper of the session to end claude cleanly at its next
// pause. the file holds the time of the request and nothing else.
func WriteStop(dir, handle string, at time.Time) error {
	path, err := stopPath(dir, handle)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, []byte(strconv.FormatInt(at.Unix(), 10)), 0o600)
}

// HasStop says whether a stop is waiting for the session.
func HasStop(dir, handle string) bool {
	path, err := stopPath(dir, handle)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// ClaimStop takes the stop request: true for the one caller that removed it.
func ClaimStop(dir, handle string) bool {
	path, err := stopPath(dir, handle)
	if err != nil {
		return false
	}
	err = os.Remove(path)
	return err == nil
}

// stopNow ends claude the way a move does, with SIGTERM and a grace, and does
// not start another one. the wrapper then ends without an error: a stop that
// was asked for is not a failure.
func (l *loop) stopNow() (bool, error) {
	if err := l.proc.Terminate(); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return false, nil
		}
		l.note("the session %s could not be stopped: %v", short(l.s.Handle), err)
		return false, nil
	}
	forced, end, err := l.waitGone()
	if l.d.Terminal != nil {
		l.d.Terminal.Restore(forced)
	}
	if end {
		return true, err
	}
	l.flush()
	return true, nil
}
