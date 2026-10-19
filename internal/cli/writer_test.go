package cli

import (
	"bytes"
	"os"
	"sync"
	"testing"
)

// TestWrapForChildReturnsFileUnchanged proves *os.File is handed back as-is:
// its writes go straight to the fd, the same path a child uses when
// inheriting it directly, so no extra locking is needed (or wanted - it would
// just be overhead on the hot, common production path).
func TestWrapForChildReturnsFileUnchanged(t *testing.T) {
	f := os.Stdout
	got, ok := wrapForChild(f).(*os.File)
	if !ok || got != f {
		t.Errorf("wrapForChild(*os.File) = %v, want the same file back unchanged", got)
	}
}

// TestWrapForChildSerializesConcurrentWrites proves a non-*os.File writer
// (e.g. the *bytes.Buffer test harnesses use) is safe for concurrent writers
// once wrapped - this is what removes the exec-spawned-copy-goroutine vs.
// printf data race documented in cmd_run_supervisor.go.
func TestWrapForChildSerializesConcurrentWrites(t *testing.T) {
	buf := &bytes.Buffer{}
	w := wrapForChild(buf)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = w.Write([]byte("x"))
		}()
	}
	wg.Wait()
	if buf.Len() != 50 {
		t.Errorf("buf.Len() = %d, want 50", buf.Len())
	}
}

// TestWrapForChildIsIdempotent proves wrapping an already-wrapped writer does
// not nest another layer - so calling it on every run (e.g. once per
// supervised restart) never builds up mutex layers.
func TestWrapForChildIsIdempotent(t *testing.T) {
	buf := &bytes.Buffer{}
	first := wrapForChild(buf)
	second := wrapForChild(first)
	if first != second {
		t.Errorf("wrapForChild(wrapForChild(w)) should return the same wrapper, got %v and %v", first, second)
	}
}
