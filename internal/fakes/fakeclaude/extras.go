package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhitalkamal/clalt/internal/fakes/fakeutil"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// runExtras does what extendFake asked for, before the session ends: a copy of
// the state, the feed, the pid, a line on stderr, or a signal sent to itself.
func runExtras(dir string) {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	for _, name := range strings.Fields(os.Getenv("FAKE_CLAUDE_EXTRAS")) {
		switch name {
		case "state":
			b, _ := fsutil.ReadFile(filepath.Join(xdg, "clalt", "state.json"))
			fakeutil.WriteFile(filepath.Join(dir, "session.state"), string(b))
		case "feed":
			files, _ := filepath.Glob(filepath.Join(xdg, "clalt", "feed", "sessions", "*.json"))
			var all []byte
			for _, p := range files {
				b, _ := fsutil.ReadFile(p)
				all = append(all, b...)
			}
			fakeutil.WriteFile(filepath.Join(dir, "session.feed"), string(all))
		case "pid":
			fakeutil.WriteFile(filepath.Join(dir, "session.pid"), strconv.Itoa(os.Getpid())+"\n")
		case "stderr":
			fmt.Fprintln(os.Stderr, "problem on stderr")
		case "signal":
			if sig := os.Getenv("FAKE_CLAUDE_SIGNAL"); sig != "" {
				fakeutil.KillSelf(sig)
			}
		}
	}
}
