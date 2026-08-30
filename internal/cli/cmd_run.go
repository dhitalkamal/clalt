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
	// resolve the account's credential into the mechanism this OS's Claude Code
	// actually reads: an injected CLAUDE_CODE_OAUTH_TOKEN env entry on macOS
	// (keychain-free), an isolated per-profile file elsewhere.
	credEnv, err := a.credentialEnv(account, dir, opts.refresh)
	if err != nil {
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

// darwinRunEnv resolves the account's token from the vault and returns it as a
// CLAUDE_CODE_OAUTH_TOKEN env entry for the child claude process. This is the
// keychain-free path on macOS: Claude Code reads CLAUDE_CODE_OAUTH_TOKEN ahead
// of the Keychain and the file, so clalt never has to touch the macOS Keychain
// (which is what caused the recurring password prompt). A missing credential is
// not fatal: return no env and let claude prompt to log in.
func (a *App) darwinRunEnv(account string, refresh bool) ([]string, error) {
	cred, _, err := a.Store.Get(account)
	if err != nil {
		a.printf("no stored credential for %q; claude will prompt you to log in\n", account)
		return nil, nil
	}
	// refresh a stale (or explicitly --refresh'd) token so the session starts
	// fresh; a single session outliving the token still needs a relaunch.
	if (refresh || cred.IsExpired(5*time.Minute)) && cred.RefreshToken != "" {
		if nc, rerr := a.refreshAccount(account); rerr == nil {
			cred = nc
		} else {
			a.printf("warning: could not refresh %q (%v); using the stored token\n", account, rerr)
		}
	}
	if cred.AccessToken == "" {
		a.printf("no usable token for %q; claude will prompt you to log in\n", account)
		return nil, nil
	}
	return []string{"CLAUDE_CODE_OAUTH_TOKEN=" + cred.AccessToken}, nil
}

// credentialEnv resolves the account's credential into the extra environment the
// child claude needs, using the mechanism that platform's Claude Code actually
// reads: an injected CLAUDE_CODE_OAUTH_TOKEN on macOS (keychain-free, so no
// password prompt), an isolated per-profile file elsewhere (which returns no
// extra env). The profile dir is created either way for settings/skills sharing.
func (a *App) credentialEnv(account, dir string, refresh bool) ([]string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if runtime.GOOS == "darwin" {
		return a.darwinRunEnv(account, refresh)
	}
	return nil, a.stageProfileCred(account, dir, refresh)
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
