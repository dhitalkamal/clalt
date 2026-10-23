package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// feedSupported says whether the global feed can be installed on goos. claude
// runs a hook command in Git Bash on Windows, or in PowerShell when Git Bash is
// not installed, and a shell command that has to work in both cannot be written
// reliably, so there it is refused instead of installed half working.
func feedSupported(goos string) error {
	if goos == "windows" {
		return fmt.Errorf("clalt feed install is not available on Windows: claude runs hook commands in Git Bash, or in PowerShell without it, and one command cannot be made reliable for both. clalt run sessions still feed clalt")
	}
	return nil
}

// stableBinary is the path the installed commands should run. a clalt that runs
// from a versioned or temporary folder (a Homebrew Cellar, a go build cache, a
// temp dir) is gone after the next upgrade or clean, so the link of the same name
// on PATH is used when it leads to the same file. the note says what it did, or
// that the path may move, and is empty when the path is fine.
func stableBinary(exe string, lookPath func(string) (string, error)) (string, string) {
	if !unstablePath(exe) {
		return exe, ""
	}
	if p, err := lookPath("clalt"); err == nil {
		if abs, err := filepath.Abs(p); err == nil && sameFile(abs, exe) {
			return abs, fmt.Sprintf("the feed runs %s, the stable path of this clalt, not %s", abs, exe)
		}
	}
	return exe, fmt.Sprintf("this clalt runs from %s, which may move or be cleaned: put clalt on PATH and install again, or run clalt feed status after an upgrade", exe)
}

// tempDirFn is where temporary files go, a variable for the tests, which live there.
var tempDirFn = os.TempDir

func unstablePath(p string) bool {
	slash := filepath.ToSlash(p)
	for _, part := range []string{"/Cellar/", "/go-build", "/.cache/go-build/"} {
		if strings.Contains(slash, part) {
			return true
		}
	}
	tmp := filepath.ToSlash(tempDirFn())
	return tmp != "" && tmp != "/" && strings.HasPrefix(slash, strings.TrimSuffix(tmp, "/")+"/")
}

func sameFile(a, b string) bool {
	fa, errA := os.Stat(a)
	fb, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(fa, fb)
}

// printNote says something about how the feed was set up, on stderr.
func (a *App) printNote(note string) { fmt.Fprintln(a.Err, "note: "+clean(note)) }
