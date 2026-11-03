package reviews

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"slices"
	"strings"
)

// Resolver finds a tool that is not on PATH, like gh or claude under a
// launchd job whose PATH is bare.
type Resolver struct {
	LookPath func(string) (string, error)
	Stat     func(string) (os.FileInfo, error)
	Home     string
	// GOOS is the platform whose rules apply, runtime.GOOS when empty
	GOOS string
}

// DefaultResolver is the Resolver backed by the real PATH and filesystem.
func DefaultResolver() Resolver {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return Resolver{LookPath: exec.LookPath, Stat: os.Stat, Home: home}
}

// Dirs lists the fallback dirs in search order. home dirs are left out when
// Home is empty, so they never turn into root-relative paths. windows has no
// homebrew or /usr/local, so there only the home dirs are searched.
func (r Resolver) Dirs() []string {
	if r.windows() {
		if r.Home == "" {
			return nil
		}
		return []string{r.join(r.Home, ".local", "bin"), r.join(r.Home, "go", "bin")}
	}
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	if r.Home != "" {
		dirs = append(dirs, r.join(r.Home, ".local", "bin"), r.join(r.Home, "go", "bin"))
	}
	return dirs
}

// Resolve returns the path to run for name. a name with a path separator is
// used as given, then PATH wins, then the first runnable regular file in a
// fallback dir.
func (r Resolver) Resolve(name string) (string, error) {
	if r.isPath(name) {
		return name, nil
	}
	p, lookErr := r.LookPath(name)
	if lookErr == nil {
		return p, nil
	}
	dirs := r.Dirs()
	for _, d := range dirs {
		for _, cand := range r.candidates(d, name) {
			info, err := r.Stat(cand)
			if err == nil && r.runnable(info) {
				return cand, nil
			}
		}
	}
	return "", fmt.Errorf("reviews: %s not found on PATH or in %s: %w", name, strings.Join(dirs, ", "), lookErr)
}

// windowsExts are the program extensions tried in a fallback dir on windows,
// where a bare name never runs.
var windowsExts = []string{".exe", ".cmd", ".bat"}

func (r Resolver) windows() bool {
	if r.GOOS != "" {
		return r.GOOS == "windows"
	}
	return runtime.GOOS == "windows"
}

// isPath matches exec.LookPath: windows takes either slash as a separator.
func (r Resolver) isPath(name string) bool {
	if r.windows() {
		return strings.ContainsAny(name, `/\`)
	}
	return strings.Contains(name, "/")
}

// join builds paths with the separator of the platform being resolved for, not
// of the host, so both rule sets can be tested anywhere.
func (r Resolver) join(elem ...string) string {
	if r.windows() {
		return strings.Join(elem, `\`)
	}
	return path.Join(elem...)
}

func (r Resolver) candidates(dir, name string) []string {
	if !r.windows() {
		return []string{r.join(dir, name)}
	}
	if ext := strings.ToLower(path.Ext(name)); ext != "" && slices.Contains(windowsExts, ext) {
		return []string{r.join(dir, name)}
	}
	out := make([]string, len(windowsExts))
	for i, ext := range windowsExts {
		out[i] = r.join(dir, name+ext)
	}
	return out
}

// runnable is a regular file with an execute bit, or on windows, which has no
// such bits, any regular file with a program extension (candidates sees to that).
func (r Resolver) runnable(info os.FileInfo) bool {
	if !info.Mode().IsRegular() {
		return false
	}
	return r.windows() || info.Mode()&0o111 != 0
}
