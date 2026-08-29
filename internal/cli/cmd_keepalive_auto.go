package cli

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/scheduler"
)

// envNoAutoKeepalive disables the automatic background keepalive entirely.
const envNoAutoKeepalive = "CLALT_NO_AUTO_KEEPALIVE"

// shouldAutoInstallKeepalive decides whether to auto-enable the background
// keepalive: only when it is not disabled (by env or by the user), not already
// handled, and there is at least one account worth keeping alive.
func shouldAutoInstallKeepalive(envDisabled, userDisabled, alreadyMarked bool, numAccounts int) bool {
	if envDisabled || userDisabled || alreadyMarked {
		return false
	}
	return numAccounts > 0
}

// ensureKeepalive auto-enables the background keepalive the first time the user
// has an account, so they never need to run `keepalive install` by hand. It is
// best-effort and quiet on failure (it simply retries on a later command), and
// it respects an explicit opt-out (env var or a prior `keepalive uninstall`).
// Safe to call after any mutating command.
func (a *App) ensureKeepalive() {
	envDisabled := os.Getenv(envNoAutoKeepalive) != ""
	userDisabled := fileExists(config.KeepaliveDisabledPath())
	marked := fileExists(config.KeepaliveAutoMarkerPath())

	n := 0
	if accts, err := a.Store.List(); err == nil {
		n = len(accts)
	}
	if !shouldAutoInstallKeepalive(envDisabled, userDisabled, marked, n) {
		return
	}
	sched, err := scheduler.For(runtime.GOOS)
	if err != nil {
		return // unsupported OS: nothing to auto-enable
	}
	// already installed (e.g. a manual install on an older clalt): record it and
	// stop re-checking on every command.
	if installed, _ := sched.Status(keepaliveLabel); installed {
		_ = touchFile(config.KeepaliveAutoMarkerPath())
		return
	}
	job, err := a.keepaliveJob(defKeepInterval, defKeepBuffer)
	if err != nil {
		return
	}
	if err := sched.Install(job); err != nil {
		return // best-effort; a later command tries again
	}
	_ = touchFile(config.KeepaliveAutoMarkerPath())
	a.printf("enabled automatic background keepalive (%s) so your tokens stay refreshed; disable with 'clalt keepalive uninstall'.\n", sched.Name())
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func touchFile(p string) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}
