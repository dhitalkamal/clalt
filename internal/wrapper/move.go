package wrapper

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/dhitalkamal/clalt/internal/pick"
	"github.com/dhitalkamal/clalt/internal/rules"
)

// tick looks at the session once. at a pause it makes the move that was asked for,
// or that auto-switch wants. exit is true when the wrapper is done, with err.
func (l *loop) tick() (exit bool, err error) {
	info, ok := l.d.Feed.Session(l.s.Handle)
	if !ok || !l.paused(info) {
		return false, nil
	}
	// with no session id there is nothing to resume, and a request is left where
	// it is until claude has told the registry
	if rec, _ := l.record(); rec.ClaudeSessionID == "" {
		return false, nil
	}
	to, why, requested := l.wanted(info)
	if to == "" {
		return false, nil
	}
	return l.move(to, why, requested, info)
}

// wanted is the account to move to, and why: the one a request names, else the
// one auto-switch picks.
func (l *loop) wanted(info SessionInfo) (to, why string, requested bool) {
	if HasRequest(l.d.Registry.Dir(), l.s.Handle) {
		req, ok, err := ClaimRequest(l.d.Registry.Dir(), l.s.Handle)
		if err != nil {
			l.refuse("", true, fmt.Sprintf("the request to move the session could not be used: %v", err))
		}
		if ok {
			return req.To, "requested with clalt move", true
		}
	}
	cands, err := l.d.Accounts.Candidates()
	if err != nil {
		return "", "", false
	}
	cur, _ := findCandidate(cands, l.account)
	dec, ok := Decide(PolicyInput{
		Now: l.d.Now(), Cfg: l.cfg, Current: cur, Candidates: cands, Limits: l.d.Feed.Limits,
		LimitHit: info.LimitHit, Cwd: l.s.Cwd, LastMove: l.lastMove, Blocked: l.blocked,
	})
	if !ok {
		return "", "", false
	}
	return dec.To.ID, dec.Why, false
}

// refuse records that a requested move did not happen. claude owns the terminal,
// so the user hears of it from the next clalt move, and from a note at the end.
func (l *loop) refuse(to string, requested bool, msg string) {
	l.note("%s", msg)
	if requested {
		_ = WriteResult(l.d.Registry.Dir(), l.s.Handle, Result{To: to, Error: msg, At: l.d.Now()})
	}
}

func findCandidate(cands []pick.Candidate, id string) (pick.Candidate, bool) {
	for _, c := range cands {
		if c.ID == id {
			return c, true
		}
	}
	return pick.Candidate{ID: id, Label: id}, false
}

// move checks that the target can take the session, and only then ends claude and
// starts the resume. everything that can fail is tried while claude still runs:
// a session whose claude is gone and whose target is not ready is lost.
func (l *loop) move(to, why string, requested bool, info SessionInfo) (exit bool, err error) {
	cands, err := l.d.Accounts.Candidates()
	if err != nil {
		l.refuse(to, requested, fmt.Sprintf("the session %s was not moved: the accounts could not be read: %v", short(l.s.Handle), err))
		return false, nil
	}
	from, _ := findCandidate(cands, l.account)
	target, known := findCandidate(cands, to)
	fail := func(format string, args ...any) (bool, error) {
		msg := fmt.Sprintf(format, args...)
		l.refuse(to, requested, fmt.Sprintf("the session %s was not moved from %s to %s: %s", short(l.s.Handle), from.Label, target.Label, msg))
		if !requested {
			l.blocked[to] = l.d.Now()
		}
		return false, nil
	}
	switch {
	case to == l.account:
		l.refuse(to, requested, fmt.Sprintf("the session %s is already running as %s", short(l.s.Handle), from.Label))
		return false, nil
	case !known:
		return fail("there is no account %s", to)
	}
	rec, _ := l.record()
	if rec.ClaudeSessionID == "" {
		l.giveBack(to, requested)
		return false, nil
	}
	for _, cwd := range cwds(l.s.Cwd, info.Cwd) {
		if !rules.Allows(target.Folders, cwd) {
			return fail("its folder rules do not include %s", cwd)
		}
	}
	prep, err := l.d.Accounts.Prepare(to, l.s.Cwd)
	if err != nil {
		return fail("%v", err)
	}
	// preparing can take a while, and the next prompt may have been typed
	if again, ok := l.d.Feed.Session(l.s.Handle); !ok || !l.paused(again) {
		l.giveBack(to, requested)
		return false, nil
	}
	select {
	case err := <-l.done:
		return true, err
	default:
	}
	return l.swap(from, target, prep, rec.ClaudeSessionID, why, fail)
}

// giveBack puts a request back for the next pause, unless a newer one is there.
func (l *loop) giveBack(to string, requested bool) {
	if requested && !HasRequest(l.d.Registry.Dir(), l.s.Handle) {
		_ = WriteRequest(l.d.Registry.Dir(), l.s.Handle, Request{To: to, RequestedAt: l.d.Now()})
	}
}

func cwds(start, now string) []string {
	if now != "" && now != start {
		return []string{start, now}
	}
	return []string{start}
}

func short(handle string) string { return handle[:8] }

// swap ends claude and starts the resume under the target. when the target
// cannot be started it resumes under the account the session was on.
func (l *loop) swap(from, target pick.Candidate, prep Prepared, sessionID, why string,
	fail func(string, ...any) (bool, error)) (bool, error) {
	if err := l.proc.Terminate(); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			// claude is already ending by itself: the user quit
			return false, nil
		}
		return fail("claude could not be asked to end: %v", err)
	}
	forced, end, err := l.waitGone()
	if l.d.Terminal != nil {
		l.d.Terminal.Restore(forced)
	}
	if end {
		return true, err
	}
	if !forced && !l.proc.EndedByTerminate(err) {
		// claude ended some other way than the signal asked: the user typed /exit
		// just before the pause was acted on, and must not get a new claude
		l.flush()
		return true, err
	}
	l.lastMove = l.d.Now()

	line := fmt.Sprintf("clalt: moved session %s from %s to %s: %s", short(l.s.Handle), from.Label, target.Label, why)
	l.flush()
	l.say(line)
	for _, n := range prep.Notes {
		l.say(n)
	}
	args := ResumeArgs(l.s.Args, sessionID)
	startErr := l.start(prep.Folder, args, Change{Account: target.ID, Folder: prep.Folder})
	if startErr == nil {
		l.account, l.folder = target.ID, prep.Folder
		return false, nil
	}
	l.say(fmt.Sprintf("clalt: could not start claude as %s: %v. resuming as %s", target.Label, startErr, from.Label))
	if err := l.start(l.folder, args, Change{}); err != nil {
		return true, fmt.Errorf("could not continue the session: %v. resume it by hand with: clalt run %s -- --resume %s",
			err, from.Label, sessionID)
	}
	return false, nil
}

func (l *loop) say(line string) {
	if l.d.Notice != nil {
		l.d.Notice(line)
	}
}

// waitGone waits for the claude that was told to end, and kills it when the
// grace is over. end says the wrapper must end too, with err: the user quit while
// it waited (the signal is passed on, and err is what claude ended with, so no new
// claude is started), or a killed claude is still there, and a second one must not
// be started beside it in the same terminal.
func (l *loop) waitGone() (forced, end bool, err error) {
	grace := time.NewTimer(l.cfg.Grace)
	defer grace.Stop()
	var killed <-chan time.Time
	for {
		select {
		case err := <-l.done:
			return forced, end, err
		case sig := <-l.d.Signals:
			if sig == syscall.SIGTERM || sig == syscall.SIGHUP {
				end = true
				l.relay(sig)
			}
		case <-grace.C:
			forced = true
			_ = l.proc.Kill()
			killed = time.After(l.cfg.KillWait)
		case <-killed:
			return forced, true, fmt.Errorf("claude did not end after it was killed (pid %d)", l.proc.PID())
		}
	}
}
