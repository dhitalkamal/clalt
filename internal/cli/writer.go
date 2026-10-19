package cli

import (
	"io"
	"os"
	"sync"
)

// syncWriter serializes concurrent writes to an underlying writer that is not
// already safe for concurrent use on its own.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// wrapForChild returns a writer safe to use as both cmd.Stdout/cmd.Stderr and
// the target of clalt's own status-line printf's while that child runs.
//
// exec.Cmd only hands a child's stdio fd straight through when Stdout/Stderr
// is a plain *os.File; for anything else (a *bytes.Buffer, as clalt's own
// tests use, or any future non-file writer) it spawns a background goroutine
// that copies the child's output into that writer via io.Copy. That copy
// races with clalt's own printf's to the same writer while the child runs
// (see cmd_run_supervisor.go's waitOrRestart) - a real, reproducible data
// race under -race, even though the writer itself (e.g. *bytes.Buffer) is
// not concurrency-safe by contract.
//
// *os.File is returned unchanged: its Write goes straight to the fd, the
// same path the child uses when inheriting it directly, so no extra locking
// is needed there (and would just be overhead on the common, hot production
// path). Anything else gets wrapped once in a shared mutex; wrapping an
// already-wrapped writer is a no-op so repeated calls (e.g. once per
// supervised restart) never stack up extra layers.
func wrapForChild(w io.Writer) io.Writer {
	if _, ok := w.(*os.File); ok {
		return w
	}
	if sw, ok := w.(*syncWriter); ok {
		return sw
	}
	return &syncWriter{w: w}
}
