package reviews

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Resolver finds a tool that is not on PATH, like gh or claude under a
// launchd job whose PATH is bare.
type Resolver struct {
	LookPath func(string) (string, error)
	Stat     func(string) (os.FileInfo, error)
	Home     string
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
// Home is empty, so they never turn into root-relative paths.
func (r Resolver) Dirs() []string {
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	if r.Home != "" {
		dirs = append(dirs, filepath.Join(r.Home, ".local", "bin"), filepath.Join(r.Home, "go", "bin"))
	}
	return dirs
}

// Resolve returns the path to run for name. a name with a path separator is
// used as given, then PATH wins, then the first executable regular file in a
// fallback dir.
func (r Resolver) Resolve(name string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) {
		return name, nil
	}
	p, lookErr := r.LookPath(name)
	if lookErr == nil {
		return p, nil
	}
	dirs := r.Dirs()
	for _, d := range dirs {
		cand := filepath.Join(d, name)
		info, err := r.Stat(cand)
		if err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return cand, nil
		}
	}
	return "", fmt.Errorf("reviews: %s not found on PATH or in %s: %w", name, strings.Join(dirs, ", "), lookErr)
}
