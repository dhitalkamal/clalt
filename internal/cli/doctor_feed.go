package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// silentAfter is how long a session may stay "starting" before doctor says its
// hooks are not reporting.
const silentAfter = 2 * time.Minute

// feedChecks looks at the session registry and the feed, and at the errors the
// hooks logged in the last day. it only reads: a stale record is removed the
// next time a session starts or sessions are listed, not here.
func (a *App) feedChecks() []checkResult {
	return []checkResult{a.sessionsCheck(), a.hookErrorsCheck()}
}

func (a *App) sessionsCheck() checkResult {
	reg := registry.Registry{Dir: config.SessionsDir()}
	entries, bad, err := reg.Scan()
	if err != nil {
		return result("feed", "sessions", statusWarn, "could not read the session records in %s: %v", config.SessionsDir(), err)
	}
	store := feed.Store{Dir: config.FeedDir()}
	states, err := store.Sessions()
	if err != nil {
		return result("feed", "sessions", statusWarn, "could not read the feed in %s: %v", config.FeedDir(), err)
	}
	labels, _ := a.labelsByID()
	now := a.clock()
	byHandle := map[string]feed.SessionState{}
	for _, s := range states {
		byHandle[s.Handle] = s
	}
	live, gone := map[string]bool{}, 0
	var silent []string
	for _, e := range entries {
		if !e.Alive {
			gone++
			continue
		}
		live[e.Handle] = true
		if st, ok := byHandle[e.Handle]; !ok || (st.State == feed.StateStarting && now.Sub(st.Since) > silentAfter) {
			who := e.Account
			if l, ok := labels[e.Account]; ok {
				who = l
			}
			silent = append(silent, fmt.Sprintf("%s (started %s ago)", who, uptime(e.StartedAt)))
		}
	}
	orphans := 0
	for _, s := range states {
		if !live[s.Handle] && !(s.State == feed.StateEnded && now.Sub(s.UpdatedAt) < 10*time.Minute) {
			orphans++
		}
	}
	var problems []string
	if len(silent) > 0 {
		problems = append(problems, fmt.Sprintf("%s has not reported: its hooks may be off (disableAllHooks, a --settings of your own, or the folder is not trusted yet)", strings.Join(silent, ", ")))
	}
	if gone > 0 || orphans > 0 {
		problems = append(problems, fmt.Sprintf("%s and %s belong to sessions that are gone, and are removed the next time a session starts",
			count(gone, "session record"), count(orphans, "feed file")))
	}
	if bad > 0 {
		problems = append(problems, count(bad, "session record")+" could not be read")
	}
	if len(problems) > 0 {
		return result("feed", "sessions", statusWarn, "%s", strings.Join(problems, ". "))
	}
	if len(live) == 0 {
		return result("feed", "sessions", statusOK, "no running sessions")
	}
	return result("feed", "sessions", statusOK, "%d running, all reporting", len(live))
}

// hookErrorsCheck counts the lines of the hook log from the last 24 hours. a
// line that does not start with a time is not one of ours and is skipped.
func (a *App) hookErrorsCheck() checkResult {
	path := filepath.Join(config.LogsDir(), hookLogName)
	f, err := os.Open(path)
	if err != nil {
		return result("feed", "hookErrors", statusOK, "no hook errors logged")
	}
	defer f.Close()
	cutoff := a.clock().Add(-24 * time.Hour)
	n, last := 0, ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		stamp, rest, _ := strings.Cut(sc.Text(), " ")
		at, err := time.Parse(time.RFC3339, stamp)
		if err != nil || at.Before(cutoff) {
			continue
		}
		n++
		last = rest
	}
	if n == 0 {
		return result("feed", "hookErrors", statusOK, "no hook errors in the last 24 hours")
	}
	return result("feed", "hookErrors", statusWarn, "%s in the last 24 hours, the last: %s (log: %s)", count(n, "hook error"), last, path)
}
