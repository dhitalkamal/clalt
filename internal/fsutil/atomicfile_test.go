package fsutil

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestWriteFileAtomicRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	want := []byte(`{"hello":"world"}`)
	if err := WriteFileAtomic(path, want, 0o600); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestWriteFileAtomicSetsPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := WriteFileAtomic(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %v, want 0600", info.Mode().Perm())
	}
}

// TestWriteFileAtomicLeavesNoTempFiles proves a successful write does not
// litter the directory with the intermediate temp file it renamed from.
func TestWriteFileAtomicLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := WriteFileAtomic(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("dir entries = %v, want just [state.json]", names)
	}
}

// TestWriteFileAtomicConcurrentWritesNeverTornRead is the core proof of the
// bug fix: a reader racing a writer must always see either the fully old
// content or the fully new content, never a truncated/partial file. Plain
// os.WriteFile truncates then writes, so a reader can observe a short or
// empty file mid-write; the rename-based approach cannot, since a rename is
// atomic from the reader's point of view - the path either resolves to the
// old inode or the new one.
func TestWriteFileAtomicConcurrentWritesNeverTornRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// payloads big enough that a naive truncate+write spans an observable
	// window (several hundred KB), each internally homogeneous so a torn
	// read is easy to detect: any read that isn't ALL 'A' or ALL 'B' (of the
	// right length) proves a torn read occurred.
	payloadA := strings.Repeat("A", 512*1024)
	payloadB := strings.Repeat("B", 256*1024)
	if err := WriteFileAtomic(path, []byte(payloadA), 0o600); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	errCh := make(chan string, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		toggle := false
		for i := 0; i < 200; i++ {
			p := payloadA
			if toggle {
				p = payloadB
			}
			toggle = !toggle
			if err := WriteFileAtomic(path, []byte(p), 0o600); err != nil {
				select {
				case errCh <- "write: " + err.Error():
				default:
				}
				return
			}
		}
		close(stop)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue // rename races an open; a miss here is fine, not a torn read
			}
			switch len(data) {
			case len(payloadA):
				if strings.Count(string(data), "A") != len(payloadA) {
					select {
					case errCh <- "torn read: wrong-length-A content did not match":
					default:
					}
					return
				}
			case len(payloadB):
				if strings.Count(string(data), "B") != len(payloadB) {
					select {
					case errCh <- "torn read: wrong-length-B content did not match":
					default:
					}
					return
				}
			default:
				select {
				case errCh <- "torn read: unexpected length " + strconv.Itoa(len(data)):
				default:
				}
				return
			}
		}
	}()

	wg.Wait()
	select {
	case msg := <-errCh:
		t.Fatal(msg)
	default:
	}
}
