package cli

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/credential"
	"github.com/dhitalkamal/clalt/internal/profile"
)

// refreshBuffer matches the buffer darwinRunEnv/resolveCred use before a run:
// refresh (and here, restart) starting this far ahead of expiry.
const refreshBuffer = 5 * time.Minute

// runSupervised wraps the child claude process in a restart loop so a long
// interactive session survives OAuth token expiry. Claude Code has no
// supported mid-session refresh for subscription (Pro/Max) profiles - it reads
// CLAUDE_CODE_OAUTH_TOKEN once at startup - so the only way to keep a session
// alive past expiry is to proactively refresh the vault token ahead of time and
// transparently restart claude with the fresh one, using --resume so the
// conversation carries over.
func (a *App) runSupervised(account, dir string, claudeArgs []string, path string, baseEnv []string, cred credential.Credential) error {
	ownedSessionID := newOwnedSessionID(claudeArgs)
	args := launchArgs(claudeArgs, ownedSessionID)

	for {
		cmd := exec.Command(path, args...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, a.Out, a.Err
		cmd.Env = append(append([]string{}, baseEnv...), "CLAUDE_CODE_OAUTH_TOKEN="+cred.AccessToken)
		if err := cmd.Start(); err != nil {
			return err
		}
		pid := cmd.Process.Pid
		cwd, _ := os.Getwd()
		_ = profile.WriteSession(config.SessionsDir(), profile.Session{
			Account: account, PID: pid, Cwd: cwd, StartedAt: time.Now(),
		})

		waitCh := make(chan error, 1)
		go func() { waitCh <- cmd.Wait() }()

		fireIn := time.Until(cred.ExpiresAtTime().Add(-refreshBuffer))
		if fireIn < 0 {
			fireIn = 0
		}
		timer := time.NewTimer(fireIn)

		runErr, restarted := a.waitOrRestart(account, pid, waitCh, timer, &cred)
		if !restarted {
			_ = profile.RemoveSession(config.SessionsDir(), pid)
			a.syncAfterRun(account, dir)
			return runErr
		}
		args = relaunchArgs(claudeArgs, ownedSessionID)
	}
}

// waitOrRestart blocks until the child exits on its own, or the refresh timer
// fires first: it then refreshes the account's token and terminates the child
// so the caller relaunches with the fresh one. cred is updated in place with
// the refreshed token when a restart happens. A refresh failure is not fatal -
// it logs a warning, retries the check a minute later, and never forces a
// restart on a token it could not actually renew.
func (a *App) waitOrRestart(account string, pid int, waitCh <-chan error, timer *time.Timer, cred *credential.Credential) (runErr error, restarted bool) {
	for {
		select {
		case runErr = <-waitCh:
			timer.Stop()
			return runErr, false
		case <-timer.C:
			nc, rerr := a.refreshAccount(account)
			if rerr != nil {
				a.printf("warning: could not refresh %q before token expiry (%v); retrying shortly\n", account, rerr)
				timer.Reset(time.Minute)
				continue
			}
			*cred = nc
			a.printf("clalt: refreshing session before token expiry, reconnecting claude...\n")
			_ = profile.Kill(config.SessionsDir(), pid)
			return <-waitCh, true
		}
	}
}

// newOwnedSessionID mints a session id for the supervisor to launch with and
// later --resume, or "" if the user is already managing continuity themselves
// via their own -r/--resume/-c/--continue/--session-id in claudeArgs.
func newOwnedSessionID(claudeArgs []string) string {
	if hasSessionFlag(claudeArgs) {
		return ""
	}
	return newUUIDv4()
}

// newUUIDv4 generates an RFC 4122 v4 UUID string. No dependency is added for
// this: it's a dozen lines over crypto/rand.
func newUUIDv4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand failing means the system RNG is broken
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// shouldSupervise reports whether `run` should wrap the child claude process
// with the transparent-restart supervisor: not explicitly disabled, not a
// one-shot -p/--print invocation (nothing to supervise once it prints and
// exits), and the account actually has a refresh token to renew with (no
// refresh token means a restart could never get a fresher one).
func shouldSupervise(opts runOpts, claudeArgs []string, cred credential.Credential) bool {
	if opts.noSupervise {
		return false
	}
	if cred.RefreshToken == "" {
		return false
	}
	for _, arg := range claudeArgs {
		if arg == "-p" || arg == "--print" {
			return false
		}
	}
	return true
}

// hasSessionFlag reports whether claudeArgs already asks claude to manage its
// own session continuity (--resume/-r/--continue/-c/--session-id), in which
// case the supervisor must not inject or rely on its own --session-id.
func hasSessionFlag(claudeArgs []string) bool {
	for _, arg := range claudeArgs {
		switch arg {
		case "--resume", "-r", "--continue", "-c", "--session-id":
			return true
		}
	}
	return false
}

// launchArgs builds the args for the first claude invocation. When the
// supervisor owns a session id (ownedSessionID != ""), it appends --session-id
// so a later supervised restart has something to --resume. Otherwise claudeArgs
// is returned unchanged (the user is managing continuity themselves).
func launchArgs(claudeArgs []string, ownedSessionID string) []string {
	if ownedSessionID == "" {
		return claudeArgs
	}
	return append(append([]string{}, claudeArgs...), "--session-id", ownedSessionID)
}

// relaunchArgs builds the args for restarting claude after a supervised token
// refresh. When the supervisor owns a session id, it appends --resume so the
// conversation carries over. Otherwise claudeArgs is returned unchanged -
// continuity may be lost, but the user's own session flags are never fought.
func relaunchArgs(claudeArgs []string, ownedSessionID string) []string {
	if ownedSessionID == "" {
		return claudeArgs
	}
	return append(append([]string{}, claudeArgs...), "--resume", ownedSessionID)
}
