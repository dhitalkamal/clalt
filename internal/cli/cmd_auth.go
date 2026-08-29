package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/dhitalkamal/clalt/internal/credential"
	"github.com/dhitalkamal/clalt/internal/oauth"
)

func (a *App) cmdLogin(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: clalt login <name>")
	}
	name := args[0]

	pkce, err := oauth.GeneratePKCE()
	if err != nil {
		return err
	}
	state, err := oauth.GenerateState()
	if err != nil {
		return err
	}
	authURL := a.OAuth.Cfg.AuthorizeURL(pkce.Challenge, state)

	a.printf("opening your browser to authorize. if it does not open, paste this URL:\n\n%s\n\n", authURL)
	openBrowser(authURL)

	pasted, err := a.prompt("after approving, paste the code shown on the page: ")
	if err != nil {
		return err
	}
	code, gotState := oauth.ParsePastedCode(pasted)
	if code == "" {
		return errors.New("no code entered")
	}
	if gotState != "" && gotState != state {
		return errors.New("state mismatch between request and pasted code; aborting for safety")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	res, err := a.OAuth.Exchange(ctx, code, pkce.Verifier, state)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	cred := credential.Credential{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresAt:    res.ExpiresAt,
		Scopes:       res.Scopes,
	}
	meta := a.labelMeta(cred)
	if err := a.Store.Save(name, cred, meta); err != nil {
		return err
	}
	a.printf("logged in and stored as %q%s\n", name, emailSuffix(meta.Email))
	a.printf("start a session as this account (no global Claude Code login needed): clalt run %s\n", name)

	if a.confirm("also make it your global Claude Code login now?") {
		if err := a.Store.Switch(name); err != nil {
			return err
		}
		a.printf("switched to %q\n", name)
	}
	return nil
}

func (a *App) cmdLogout(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: clalt logout <name>")
	}
	name := args[0]
	if !a.confirm(fmt.Sprintf("forget stored credential for %q?", name)) {
		a.printf("aborted\n")
		return nil
	}
	if err := a.Store.Delete(name); err != nil {
		return err
	}
	a.printf("forgot %q (Claude Code's own live login is untouched unless you switch)\n", name)
	return nil
}

func (a *App) cmdRefresh(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: clalt refresh <name> | clalt refresh --all")
	}
	if args[0] == "--all" {
		return a.refreshAll()
	}
	name := args[0]
	cred, err := a.refreshAccount(name)
	if err != nil {
		return err
	}
	a.printf("refreshed %q, token now valid %s\n", name, humanUntil(cred.ExpiresAtTime()))
	return nil
}

// refreshAll forces a refresh of every account that has a refresh token,
// regardless of how far off expiry is. Errors are collected so one bad account
// does not stop the rest.
func (a *App) refreshAll() error {
	accts, err := a.Store.List()
	if err != nil {
		return err
	}
	var failed int
	for _, ac := range accts {
		cred, _, gerr := a.Store.Get(ac.Name)
		if gerr != nil || cred.RefreshToken == "" {
			continue
		}
		nc, rerr := a.refreshAccount(ac.Name)
		if rerr != nil {
			failed++
			a.printf("failed %q: %v\n", ac.Name, rerr)
			continue
		}
		a.printf("refreshed %q, token now valid %s\n", ac.Name, humanUntil(nc.ExpiresAtTime()))
	}
	if failed > 0 {
		return fmt.Errorf("%d account(s) failed to refresh", failed)
	}
	return nil
}

// refreshAccount refreshes one account's token, persists the rotated set, and
// (if that account is active) pushes the fresh token into Claude Code's live
// store. It is the single refresh path shared by cmdRefresh, refreshAll, and
// keepalive.
func (a *App) refreshAccount(name string) (credential.Credential, error) {
	cred, meta, err := a.Store.Get(name)
	if err != nil {
		return cred, err
	}
	if cred.RefreshToken == "" {
		return cred, fmt.Errorf("account %q has no refresh token", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	res, err := a.doRefresh(ctx, cred.RefreshToken)
	if err != nil {
		return cred, fmt.Errorf("refresh failed: %w", err)
	}
	cred.AccessToken = res.AccessToken
	cred.RefreshToken = res.RefreshToken // rotated: must persist
	cred.ExpiresAt = res.ExpiresAt
	if len(res.Scopes) > 0 {
		cred.Scopes = res.Scopes
	}
	if err := a.Store.Save(name, cred, meta); err != nil {
		return cred, err
	}
	// if this is the active account, push the fresh token into the live store too
	if active, _ := a.Store.Active(); active == name {
		if err := a.Live.Write(cred); err != nil {
			return cred, err
		}
	}
	return cred, nil
}

// openBrowser best-effort opens a URL on the current OS. CLALT_NO_BROWSER
// disables it (used by scripted end-to-end runs). The URL is always printed by
// the caller too, so a missing opener is not fatal.
func openBrowser(url string) {
	if os.Getenv("CLALT_NO_BROWSER") != "" {
		return
	}
	name, args := browserCommand(runtime.GOOS, url)
	if _, err := exec.LookPath(name); err != nil {
		return
	}
	_ = exec.Command(name, args...).Start()
}

// browserCommand picks the OS opener and its args: `open` on macOS, `rundll32`
// on Windows, `xdg-open` on Linux and other unixes.
func browserCommand(goos, url string) (name string, args []string) {
	switch goos {
	case "darwin":
		return "open", []string{url}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		return "xdg-open", []string{url}
	}
}
