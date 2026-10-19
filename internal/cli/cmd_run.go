package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/credential"
	"github.com/dhitalkamal/clalt/internal/profile"
)

// cmdRun launches Claude Code for one account. On Linux/Windows the account runs
// in an isolated config dir in parallel with others. On macOS Claude Code reads
// one global credential regardless of config dir, so run loads the account into
// the live login before launching (accounts are not parallel there).
// Usage: clalt run <account> [--isolated] [--refresh] [--no-supervise] [-- claude args]
func (a *App) cmdRun(args []string) error {
	account, opts, claudeArgs, err := parseRunArgs(args)
	if err != nil {
		return err
	}

	dir := config.ProfileDir(account)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	// exec spawns a background copy goroutine to relay a child's output
	// whenever Stdout/Stderr isn't a plain *os.File, which would otherwise
	// race with clalt's own status-line printf's to the same writer while the
	// child runs (a real -race failure, see writer.go); *os.File itself needs
	// no extra locking, so this is a no-op in normal (terminal) operation.
	a.Out = wrapForChild(a.Out)
	a.Err = wrapForChild(a.Err)

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

	// resolve the account's credential into the mechanism this OS's Claude Code
	// actually reads: an injected CLAUDE_CODE_OAUTH_TOKEN env entry on macOS
	// (keychain-free), an isolated per-profile file elsewhere.
	var credEnv []string
	if runtime.GOOS == "darwin" {
		cred, ok := a.resolveCred(account, opts.refresh)
		// macOS has no supported mid-session OAuth refresh (Claude Code reads
		// CLAUDE_CODE_OAUTH_TOKEN once at startup), so a long interactive session
		// otherwise hard-fails with "Login expired" once the token expires. The
		// supervisor proactively refreshes and transparently restarts claude
		// before that happens; see runSupervised.
		if ok && shouldSupervise(opts, claudeArgs, cred) {
			baseEnv := append(os.Environ(),
				"CLAUDE_CONFIG_DIR="+dir,
				"CLAUDE_SECURESTORAGE_CONFIG_DIR="+dir,
			)
			a.printf("launching claude as %q (profile: %s)\n", account, dir)
			return a.runSupervised(account, dir, claudeArgs, path, baseEnv, cred)
		}
		if ok {
			credEnv = []string{"CLAUDE_CODE_OAUTH_TOKEN=" + cred.AccessToken}
		}
		// !ok: credEnv stays nil and claude prompts to log in; resolveCred
		// already printed the reason, so don't resolve (and re-warn) again.
	} else if err := a.stageProfileCred(account, dir, opts.refresh); err != nil {
		return err
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
	// CLAUDE_CODE_OAUTH_TOKEN (macOS) makes this the sole login for the child,
	// so accounts run in parallel without sharing one global keychain login.
	cmd.Env = append(cmd.Env, credEnv...)
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

// resolveCred fetches the account's stored credential and, if refresh is
// requested or the token is within 5 minutes of expiry, refreshes it first.
// This is the keychain-free path on macOS: the resolved token is handed to the
// child claude process via CLAUDE_CODE_OAUTH_TOKEN (which Claude Code reads
// ahead of the Keychain and the file), so clalt never has to touch the macOS
// Keychain (which is what caused the recurring password prompt). ok is false
// if there is no stored credential or no usable access token (claude will
// prompt to log in itself in that case).
func (a *App) resolveCred(account string, refresh bool) (cred credential.Credential, ok bool) {
	cred, _, err := a.Store.Get(account)
	if err != nil {
		a.printf("no stored credential for %q; claude will prompt you to log in\n", account)
		return credential.Credential{}, false
	}
	// refresh a stale (or explicitly --refresh'd) token so the session starts
	// fresh; a single session outliving the token still needs a relaunch,
	// which is exactly what runSupervised handles for the darwin path.
	if (refresh || cred.IsExpired(5*time.Minute)) && cred.RefreshToken != "" {
		if nc, rerr := a.refreshAccount(account); rerr == nil {
			cred = nc
		} else {
			a.printf("warning: could not refresh %q (%v); using the stored token\n", account, rerr)
		}
	}
	if cred.AccessToken == "" {
		a.printf("no usable token for %q; claude will prompt you to log in\n", account)
		return credential.Credential{}, false
	}
	return cred, true
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
// On macOS the token was injected via env and Claude Code manages its own token
// rotation out of band, so there is nothing for clalt to capture (keepalive keeps
// the vault fresh); elsewhere the rotated token is read back from the profile file.
func (a *App) syncAfterRun(account, dir string) {
	if runtime.GOOS == "darwin" {
		return
	}
	a.syncProfileBack(account, dir)
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
	refresh     bool // re-stage the stored credential and re-copy shared settings
	isolated    bool // do NOT share the user's main config into the profile
	noSupervise bool // disable the transparent-restart supervisor (macOS long sessions)
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
		case arg == "--no-supervise":
			opts.noSupervise = true
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
		return "", runOpts{}, nil, errors.New("usage: clalt run <account> [--isolated] [--refresh] [--no-supervise] [-- claude args]")
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
