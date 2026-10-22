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

// NewApp builds an App backed by on-disk state. clalt is keychain-free: the live
// store is file-only on every platform (nil keychain), so reading the current
// login for `add` never prompts, and multi-account sessions run via `clalt run`,
// which injects CLAUDE_CODE_OAUTH_TOKEN so Claude Code never consults the macOS
// Keychain at all. clalt's own per-account secrets live in a local encrypted
// vault, not any OS keychain; there is no more legacy macOS Keychain storage to
// migrate from.
func NewApp() (*App, error) {
	// file-only live store: never touches the macOS Keychain (nil keychain)
	live := livestore.New(nil, config.ClaudeCredentialsPath())

	vault, err := keychain.NewEncFileStore(config.VaultDir(), config.VaultKeyPath())
	if err != nil {
		return nil, fmt.Errorf("open credential vault: %w", err)
	}

	return &App{
		Store:   account.New(vault, config.StatePath(), config.SwitchLogPath()),
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
