package fsutil

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolvePath returns an absolute path with symlinks resolved. only the part of
// p that exists is resolved; the rest is appended as written, so a folder that
// has not been created yet still compares correctly. a dangling symlink counts
// as missing and is left as is.
func ResolvePath(p string) string {
	if p == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	}
	tail := ""
	for cur := abs; ; {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(r, tail)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs
		}
		tail = filepath.Join(filepath.Base(cur), tail)
		cur = parent
	}
}

// SamePath reports whether a and b lead to the same place. when both exist it
// asks the file system (os.SameFile), which also sees through symlinks and the
// case folding of macOS and Windows; otherwise it compares resolved paths. an
// empty path never matches anything.
func SamePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if fa, err := os.Stat(a); err == nil {
		if fb, err := os.Stat(b); err == nil {
			return os.SameFile(fa, fb)
		}
	}
	return ResolvePath(a) == ResolvePath(b)
}

// Within reports whether p is dir or lies below it, with the same rules as
// SamePath: an existing ancestor of p that is the same file as dir counts, so a
// symlinked or differently cased spelling is inside too. when dir does not
// exist it falls back to comparing resolved paths.
func Within(p, dir string) bool {
	if p == "" || dir == "" {
		return false
	}
	if dfi, err := os.Stat(dir); err == nil {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = filepath.Clean(p)
		}
		for cur := abs; ; {
			if fi, err := os.Stat(cur); err == nil && os.SameFile(fi, dfi) {
				return true
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
	}
	rel, err := filepath.Rel(ResolvePath(dir), ResolvePath(p))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
