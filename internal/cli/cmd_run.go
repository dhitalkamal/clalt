package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
)

// cmdRun launches Claude Code for one account. On Linux/Windows the account runs
// in an isolated config dir in parallel with others. On macOS Claude Code reads
// one global credential regardless of config dir, so run loads the account into
// the live login before launching (accounts are not parallel there).
// Usage: clalt run <account> [--isolated] [--refresh] [-- claude args]
func (a *App) cmdRun(args []string) error {
	account, opts, claudeArgs, err := parseRunArgs(args)
	if err != nil {
		return err
	}

	dir := config.ProfileDir(account)
	// put the account's credential where claude will actually read it on this OS
	if err := a.stageForRun(account, dir, opts.refresh); err != nil {
		return err
	}

	// by default a profile inherits the user's full Claude config (settings,
	// skills, MCP, CLAUDE.md, ...) so a run session behaves like their normal
	// setup; --isolated opts out for a clean-room profile.
	if !opts.isolated {
		if shared, serr := profile.ShareConfig(dir, config.ClaudeConfigDir(), opts.refresh); serr == nil && len(shared) > 0 {
			a.printf("shared config into profile: %v\n", shared)
		}
		// MCP servers live in ~/.claude.json, outside the dirs ShareConfig covers,
		// so project them separately into the profile's own .claude.json.
		if servers, merr := profile.LinkMCP(dir, config.ClaudeJSONPath()); merr == nil && len(servers) > 0 {
			a.printf("shared mcp servers into profile: %v\n", servers)
		}
	}

	bin := claudeBinary()
	path, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("could not find the %q binary on PATH: %w", bin, err)
	}

	a.printf("launching claude as %q (profile: %s)\n", account, dir)
	cmd := exec.Command(path, claudeArgs...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, a.Out, a.Err
	// point Claude Code at the profile dir for settings/skills; on Linux/Windows
	// this also scopes the credential file, giving true isolation.
	cmd.Env = append(os.Environ(),
		"CLAUDE_CONFIG_DIR="+dir,
		"CLAUDE_SECURESTORAGE_CONFIG_DIR="+dir,
	)
	if err := cmd.Start(); err != nil {
		return err
	}
	// record the live session so `clalt sessions` and the dashboard can show it
	pid := cmd.Process.Pid
	cwd, _ := os.Getwd()
	_ = profile.WriteSession(config.SessionsDir(), profile.Session{
		Account: account, PID: pid, Cwd: cwd, StartedAt: time.Now(),
	})
	runErr := cmd.Wait()
	_ = profile.RemoveSession(config.SessionsDir(), pid)

	// capture any rotated token back into the account slot (best-effort)
	a.syncAfterRun(account, dir)
	return runErr
}

// stageForRun makes the account's credential visible to the claude session it is
// about to launch, using the mechanism that platform's Claude Code actually
// reads: the live keychain on macOS, an isolated per-profile file elsewhere.
func (a *App) stageForRun(account, dir string, refresh bool) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		// macOS Claude Code reads the one global keychain item and ignores the
		// per-profile .credentials.json, so load the account into the live store
		// the way switch does. Consequence: accounts cannot run in parallel on
		// macOS, they share a single login.
		if _, _, err := a.Store.Get(account); err != nil {
			a.printf("no stored credential for %q; claude will prompt you to log in\n", account)
			return nil
		}
		if a.hasOtherRunSession() {
			a.printf("note: on macOS all sessions share one Claude login; another run is active, so this changes the shared login to %q\n", account)
		}
		if err := a.Store.Switch(account); err != nil {
			return fmt.Errorf("activate %q into the live login: %w", account, err)
		}
		return nil
	}
	return a.stageProfileCred(account, dir, refresh)
}

// stageProfileCred writes the account's stored credential into the isolated
// profile dir (Linux/Windows), where Claude Code reads it per CLAUDE_CONFIG_DIR.
// A missing credential is not fatal: claude will prompt to log in in the profile.
func (a *App) stageProfileCred(account, dir string, refresh bool) error {
	if profile.HasCredential(dir) && !refresh {
		return nil // keep the profile's own (possibly rotated) credential
	}
	cred, _, err := a.Store.Get(account)
	if err != nil {
		a.printf("no stored credential for %q; claude will prompt you to log in inside the isolated profile\n", account)
		return nil
	}
	return profile.Stage(dir, cred)
}

// syncAfterRun captures any token the session rotated back into the account slot.
// On macOS the rotation lands in the live store; elsewhere in the profile file.
func (a *App) syncAfterRun(account, dir string) {
	if runtime.GOOS == "darwin" {
		cred, err := a.Live.Read()
		if err != nil || cred.AccessToken == "" {
			return
		}
		if _, meta, gerr := a.Store.Get(account); gerr == nil {
			_ = a.Store.Save(account, cred, meta)
		}
		return
	}
	a.syncProfileBack(account, dir)
}

// hasOtherRunSession reports whether another `clalt run` session is already live,
// used to warn that macOS cannot isolate accounts.
func (a *App) hasOtherRunSession() bool {
	sessions, err := profile.ListSessions(config.SessionsDir())
	return err == nil && len(sessions) > 0
}

// syncProfileBack copies a profile's current (possibly rotated) credential back
// into the account slot so the dashboard and `switch` stay current.
func (a *App) syncProfileBack(account, dir string) {
	cred, ok, err := profile.Read(dir)
	if err != nil || !ok {
		return
	}
	if _, meta, gerr := a.Store.Get(account); gerr == nil {
		_ = a.Store.Save(account, cred, meta)
	}
}

// runOpts are the flags accepted by `clalt run`.
type runOpts struct {
	refresh  bool // re-stage the stored credential and re-copy shared settings
	isolated bool // do NOT share the user's main config into the profile
}

// parseRunArgs pulls the account name, clalt flags, and passthrough claude args
// (everything after an optional `--`) out of the argument list. --link is still
// accepted for backward compatibility but is now the default, so it is a no-op.
func parseRunArgs(args []string) (account string, opts runOpts, claudeArgs []string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			claudeArgs = append(claudeArgs, args[i+1:]...)
			i = len(args)
		case arg == "--refresh":
			opts.refresh = true
		case arg == "--isolated":
			opts.isolated = true
		case arg == "--link":
			// sharing is the default now; accept the old flag as a no-op
		case account == "":
			account = arg
		default:
			// once we have the account, remaining args pass through to claude
			claudeArgs = append(claudeArgs, arg)
		}
	}
	if account == "" {
		return "", runOpts{}, nil, errors.New("usage: clalt run <account> [--isolated] [--refresh] [-- claude args]")
	}
	return account, opts, claudeArgs, nil
}

// claudeBinary allows tests/power users to override the launched binary.
func claudeBinary() string {
	if b := os.Getenv("CLALT_CLAUDE_BIN"); b != "" {
		return b
	}
	return "claude"
}
