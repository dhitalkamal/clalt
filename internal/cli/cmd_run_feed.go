package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// sessionEnvVar carries the handle of a clalt run session to claude, and from
// there to its hooks and its statusline.
const sessionEnvVar = "CLALT_SESSION"

// runSession is what clalt run keeps about one session while claude runs: the
// record in the registry, which is written before claude starts and removed
// after it ends.
type runSession struct {
	reg     registry.Registry
	store   feed.Store
	handle  string
	account string
	now     func() time.Time
	ok      bool // the record was written
}

// beginSession writes the record of a session that is about to start. a record
// that cannot be written is a warning: the session still runs, only the commands
// that look for it will not see it.
func (a *App) beginSession(acc account.Account, folder string) *runSession {
	s := &runSession{
		reg: registry.Registry{Dir: config.SessionsDir()}, store: feed.Store{Dir: config.FeedDir()},
		handle: registry.NewHandle(), account: acc.Name, now: a.clock,
	}
	cwd, _ := os.Getwd()
	err := s.reg.Create(registry.Record{
		Handle: s.handle, Account: acc.Name, Folder: folder, WrapperPID: os.Getpid(), Cwd: cwd,
		Terminal: registry.DetectTerminal(os.Getenv), StartedAt: a.clock(),
	})
	if err != nil {
		fmt.Fprintf(a.Err, "warning: could not record the session: %v\n", cleanErr(err))
		return s
	}
	s.ok = true
	// the feed starts at "starting" and the hooks take it from there. what is
	// left of sessions that are gone goes now, while the registry is at hand
	if err := s.store.UpdateSession(s.handle, s.account, a.clock(), func(*feed.SessionState) bool { return true }); err != nil {
		fmt.Fprintf(a.Err, "warning: could not write the session feed: %v\n", cleanErr(err))
	}
	pruneFeed(s.store, s.reg, a.clock())
	return s
}

// withFeedSettings puts the --settings that installs the hooks and the
// statusline in front of claude's arguments. a --settings of the user's own
// wins, since claude takes one, and then there is no feed for the session.
func (a *App) withFeedSettings(args []string) []string {
	if hasSettingsFlag(args) {
		fmt.Fprintln(a.Err, "note: you passed --settings, so the clalt feed is off for this session")
		return args
	}
	bin, err := os.Executable()
	if err == nil {
		bin, err = filepath.Abs(bin)
	}
	if err != nil {
		fmt.Fprintf(a.Err, "warning: the clalt feed is off for this session, clalt could not find itself: %v\n", cleanErr(err))
		return args
	}
	var user *userStatusLine
	if u, ok := readUserStatusLine(); ok {
		user = &u
	}
	settings, err := feedSettings(bin, user)
	if err != nil {
		fmt.Fprintf(a.Err, "warning: the clalt feed is off for this session: %v\n", cleanErr(err))
		return args
	}
	return append([]string{"--settings", settings}, args...)
}

// attach hands the handle to claude, replacing one the shell already had (a
// clalt run inside a session that clalt started).
func (s *runSession) attach(cmd *exec.Cmd) {
	cmd.Env = setEnv(cmd.Env, sessionEnvVar, s.handle)
}

// started records claude's pid, which is known only after the start.
func (s *runSession) started(pid int) {
	if s.ok {
		_, _, _ = s.reg.Update(s.handle, func(r *registry.Record) { r.PID = pid })
	}
}

// finish removes the record. it is called whether or not claude started.
func (s *runSession) finish() {
	if !s.ok {
		return
	}
	// claude may be gone without a SessionEnd, so the end is written here too
	_ = s.store.UpdateSession(s.handle, s.account, s.now(), func(st *feed.SessionState) bool {
		if st.State == feed.StateEnded {
			return false
		}
		st.State, st.Since, st.Reason = feed.StateEnded, s.now(), ""
		return true
	})
	_ = s.reg.Remove(s.handle)
}

// setEnv returns env with name set to value and no other value for it.
func setEnv(env []string, name, value string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if k, _, _ := strings.Cut(kv, "="); k != name {
			out = append(out, kv)
		}
	}
	return append(out, name+"="+value)
}
