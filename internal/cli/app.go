// Package cli wires the clalt packages together and dispatches subcommands.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/keychain"
	"github.com/dhitalkamal/clalt/internal/livestore"
	"github.com/dhitalkamal/clalt/internal/oauth"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// defaultAPIBase is the host for best-effort live usage and profile lookups,
// overridable via CLALT_API_BASE.
const defaultAPIBase = "https://api.anthropic.com"

// EnvAPIBase overrides the API host for live usage and profile lookups.
const EnvAPIBase = "CLALT_API_BASE"

func apiBaseFromEnv() string {
	if v := os.Getenv(EnvAPIBase); v != "" {
		return v
	}
	return defaultAPIBase
}

// App holds the constructed dependencies for one invocation.
type App struct {
	Store *account.Store
	Live  *livestore.Store
	KC    keychain.Store
	OAuth *oauth.Client
	Cache *usage.Cache

	// ApiBase is the host for best-effort profile and live-usage calls. Empty
	// disables those network lookups (used by tests).
	ApiBase string

	JSON bool
	Out  io.Writer
	In   io.Reader
	Err  io.Writer

	// refreshToken exchanges a refresh token for a rotated set. Nil in normal
	// operation (falls back to OAuth.Refresh); injectable so token-refresh flows
	// can be tested without hitting the network.
	refreshToken func(ctx context.Context, refreshToken string) (oauth.TokenResult, error)

	reader *bufio.Reader // shared line reader for prompts
}

// doRefresh runs the injected refresh func if set, else the real OAuth client.
func (a *App) doRefresh(ctx context.Context, refresh string) (oauth.TokenResult, error) {
	if a.refreshToken != nil {
		return a.refreshToken(ctx, refresh)
	}
	return a.OAuth.Refresh(ctx, refresh)
}

// NewApp builds an App backed by on-disk state. Claude Code's own live
// credential lives in the macOS Keychain on darwin (mirrored to the file), and
// in the ~/.claude/.credentials.json file alone on Linux/Windows, matching what
// Claude Code itself reads on each platform: keychain.NewSystem() returns nil
// off macOS, which puts the live store in file-only mode. clalt's own per-account
// secrets always live in a local encrypted vault instead of any OS keychain, so
// listing accounts, switching, and the keepalive background agent never trigger
// a Keychain password prompt. Any secrets left over from clalt's old
// Keychain-only storage are migrated into the vault once (macOS only).
func NewApp() (*App, error) {
	sysKC := keychain.NewSystem()
	live := livestore.New(sysKC, config.ClaudeCredentialsPath())

	vault, err := keychain.NewEncFileStore(config.VaultDir(), config.VaultKeyPath())
	if err != nil {
		return nil, fmt.Errorf("open credential vault: %w", err)
	}
	if _, err := keychain.MigrateVault(sysKC, vault, account.KeychainService); err != nil {
		return nil, fmt.Errorf("migrate credential vault: %w", err)
	}

	return &App{
		Store:   account.New(vault, live, config.StatePath(), config.SwitchLogPath()),
		Live:    live,
		KC:      vault,
		OAuth:   oauth.NewClient(),
		Cache:   usage.NewCache(config.CacheDir()),
		ApiBase: apiBaseFromEnv(),
		Out:     os.Stdout,
		In:      os.Stdin,
		Err:     os.Stderr,
	}, nil
}
