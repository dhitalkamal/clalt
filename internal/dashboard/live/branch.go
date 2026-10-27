package live

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// maxUp is how many folders gitBranch climbs looking for .git
	maxUp = 24
	// maxHead is the most that is read of a HEAD or .git file: a real one is
	// under 300 bytes, and anything longer is not one
	maxHead = 512
	// maxBranch is the longest branch name shown
	maxBranch = 200
)

// gitBranch is the branch checked out at dir, read from .git/HEAD without
// running git. a detached head gives its first 7 characters, and nothing gives
// "": a branch is a nicety, never worth a failure or a wait.
func gitBranch(dir string) string {
	if dir == "" {
		return ""
	}
	for i := 0; i < maxUp; i++ {
		if head, found := headOf(filepath.Join(dir, ".git")); found {
			return branchFromHead(head)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// readSmall reads a regular file of at most maxHead bytes. anything else (a
// pipe that would block, a device, a symlink, a file that is too big) is "not
// there", and the open does not wait for a writer.
func readSmall(path string) (string, bool) {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return "", false
	}
	f, err := os.OpenFile(path, os.O_RDONLY|nonBlock, 0)
	if err != nil {
		return "", false
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxHead+1))
	if err != nil || len(raw) > maxHead {
		return "", false
	}
	return string(raw), true
}

// headOf reads HEAD of a .git folder, or of the folder a worktree's .git file
// points at. found says there is a .git here, even when its HEAD is unusable,
// so the climb stops at the nearest one.
func headOf(dotGit string) (head string, found bool) {
	fi, err := os.Lstat(dotGit)
	if err != nil {
		return "", false
	}
	if fi.IsDir() {
		head, _ = readSmall(filepath.Join(dotGit, "HEAD"))
		return head, true
	}
	raw, ok := readSmall(dotGit)
	if !ok {
		return "", fi.Mode().IsRegular()
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(raw), "gitdir: ")
	if !ok {
		return "", true
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(dotGit), target)
	}
	// the target is whatever the file says: it must hold a HEAD that is a
	// small regular file, which readSmall checks, to count as a git folder
	head, _ = readSmall(filepath.Join(target, "HEAD"))
	return head, true
}

func branchFromHead(head string) string {
	head = strings.TrimSpace(head)
	if ref, ok := strings.CutPrefix(head, "ref: refs/heads/"); ok {
		if len(ref) > maxBranch || strings.ContainsAny(ref, "\x00\n") {
			return ""
		}
		return ref
	}
	if len(head) >= 7 && !strings.ContainsAny(head, " \t:") {
		return head[:7]
	}
	return ""
}
