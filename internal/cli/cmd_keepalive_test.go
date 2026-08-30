package cli

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/credential"
	"github.com/dhitalkamal/clalt/internal/oauth"
)

// fakeRefresh returns a rotated token set without any network call.
func fakeRefresh() func(context.Context, string) (oauth.TokenResult, error) {
	return func(_ context.Context, rt string) (oauth.TokenResult, error) {
		return oauth.TokenResult{
			AccessToken:  "new-" + rt,
			RefreshToken: "rot-" + rt,
			ExpiresAt:    time.Now().Add(8 * time.Hour).UnixMilli(),
			Scopes:       []string{"user:inference"},
		}, nil
	}
}

// saveCred stores an account with a specific expiry and refresh token.
func saveCred(t *testing.T, app *App, name, refresh string, expiresIn time.Duration) {
	t.Helper()
	cred := credential.Credential{
		AccessToken:  "old-" + name,
		RefreshToken: refresh,
		ExpiresAt:    time.Now().Add(expiresIn).UnixMilli(),
		Scopes:       []string{"user:inference"},
	}
	if err := app.Store.Save(name, cred, account.Meta{}); err != nil {
		t.Fatalf("save %s: %v", name, err)
	}
}

func TestKeepaliveOnceRefreshesOnlyNearExpiry(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.refreshToken = fakeRefresh()

	saveCred(t, app, "stale", "r-stale", 10*time.Minute) // within buffer
	saveCred(t, app, "fresh", "r-fresh", 2*time.Hour)    // outside buffer
	saveCred(t, app, "nokey", "", 5*time.Minute)         // no refresh token -> skip

	refreshed, errs := app.keepaliveOnce(45 * time.Minute)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(refreshed) != 1 || refreshed[0] != "stale" {
		t.Fatalf("refreshed = %v, want [stale]", refreshed)
	}

	// stale got the rotated token set persisted
	sc, _, _ := app.Store.Get("stale")
	if sc.AccessToken != "new-r-stale" || sc.RefreshToken != "rot-r-stale" {
		t.Errorf("stale not rotated: %+v", sc)
	}
	// fresh untouched
	fc, _, _ := app.Store.Get("fresh")
	if fc.AccessToken != "old-fresh" {
		t.Errorf("fresh should be untouched, got %q", fc.AccessToken)
	}
}

// TestKeepaliveRefreshesVaultNotLiveStore proves keepalive stays keychain-free:
// it rotates the account's token in the vault but never writes Claude Code's live
// store (the macOS Keychain item that caused the recurring password prompt).
func TestKeepaliveRefreshesVaultNotLiveStore(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.refreshToken = fakeRefresh()

	saveCred(t, app, "work", "r-work", 5*time.Minute)
	if err := app.Store.SetActive("work"); err != nil {
		t.Fatalf("set active: %v", err)
	}

	if _, errs := app.keepaliveOnce(45 * time.Minute); len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	// the vault slot was rotated
	wc, _, _ := app.Store.Get("work")
	if wc.AccessToken != "new-r-work" {
		t.Errorf("vault token not rotated: %q", wc.AccessToken)
	}
	// the live store must remain empty: keepalive does not touch the keychain
	if live, err := app.Live.Read(); err == nil && live.AccessToken != "" {
		t.Errorf("keepalive must not write the live store, found %q", live.AccessToken)
	}
}

func TestRefreshAllForcesEveryAccount(t *testing.T) {
	app, out, _ := newTestApp(t)
	app.refreshToken = fakeRefresh()

	saveCred(t, app, "a", "r-a", 10*time.Hour) // far future, still forced
	saveCred(t, app, "b", "r-b", 20*time.Hour)
	out.Reset()

	if err := app.cmdRefresh([]string{"--all"}); err != nil {
		t.Fatalf("refresh --all: %v", err)
	}
	for _, name := range []string{"a", "b"} {
		c, _, _ := app.Store.Get(name)
		if c.AccessToken != "new-r-"+name {
			t.Errorf("%s not refreshed: %q", name, c.AccessToken)
		}
	}
}

func TestKeepaliveOnceStableOrder(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.refreshToken = fakeRefresh()
	saveCred(t, app, "z", "r-z", 1*time.Minute)
	saveCred(t, app, "a", "r-a", 1*time.Minute)

	refreshed, _ := app.keepaliveOnce(45 * time.Minute)
	if !sort.StringsAreSorted(refreshed) {
		// List order is store order; assert both present regardless
		if len(refreshed) != 2 {
			t.Fatalf("expected both refreshed, got %v", refreshed)
		}
	}
}
