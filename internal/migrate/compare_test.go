package migrate

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

func TestCompareFilesReportsAReadError(t *testing.T) {
	bad := iotest.ErrReader(errBoom)
	for name, c := range map[string]struct{ src, other io.Reader }{
		"the source":     {bad, strings.NewReader("abc")},
		"the other file": {strings.NewReader("abc"), bad},
	} {
		if _, _, err := compareFiles(c.src, c.other, 3, 3); !errors.Is(err, errBoom) {
			t.Errorf("%s: err = %v, want the read error", name, err)
		}
	}
}

// a file that is longer than the source cannot be its start or the same, so
// nothing needs reading to say so.
func TestCompareFilesNeedsNoReadWhenTheOtherIsLonger(t *testing.T) {
	bad := iotest.ErrReader(errBoom)
	res, reason, err := compareFiles(bad, bad, 3, 4)
	if err != nil || res != differs || reason != ReasonDiffers {
		t.Errorf("got %v %q %v, want differs without reading", res, reason, err)
	}
}

func TestCompareOpenReportsAStatError(t *testing.T) {
	dir := t.TempDir()
	open := put(t, dir, "open.txt", "x")
	shut := put(t, dir, "shut.txt", "x")
	a, _ := os.Open(open)
	b, _ := os.Open(shut)
	defer a.Close()
	b.Close()

	if _, _, err := compareOpen(a, b); !errors.Is(err, os.ErrClosed) {
		t.Errorf("other closed: err = %v, want a closed file error", err)
	}
	if _, _, err := compareOpen(b, a); !errors.Is(err, os.ErrClosed) {
		t.Errorf("source closed: err = %v, want a closed file error", err)
	}
}

func TestCompareOnDiskReportsAMissingFile(t *testing.T) {
	dir := t.TempDir()
	there := put(t, dir, "there.txt", "x")
	gone := filepath.Join(dir, "gone.txt")

	if _, _, err := compareOnDisk(gone, there); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing source: err = %v", err)
	}
	if _, _, err := compareOnDisk(there, gone); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing other: err = %v", err)
	}
}
