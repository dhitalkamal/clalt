# clalt

Claude alt-account manager. Manage multiple Claude accounts for Claude Code from
one CLI: authenticate accounts, launch keychain-free sessions for any of them
(in parallel), and see a per-account usage dashboard. macOS, Linux, and Windows.

Unofficial community tool, not affiliated with Anthropic.

## How it works

clalt is keychain-free. It stores any number of named credential slots in its own
local encrypted vault (`~/.config/clalt/vault`, AES-256-GCM) and never touches the
macOS Keychain. Non-secret metadata (email, plan, timestamps) and the active
pointer live in `~/.config/clalt/state.json`.

You use an account by launching a session for it with `clalt run <name>`. clalt
resolves that account's token from the vault (refreshing it first if needed) and
hands it to the child `claude` process through the `CLAUDE_CODE_OAUTH_TOKEN`
environment variable. Claude Code reads that variable ahead of the Keychain and
the credentials file, so:

- clalt never reads or writes the macOS Keychain, so it never triggers the
  recurring login-password prompt.
- Different accounts run in parallel: each `clalt run` gets its own token in its
  own child environment, plus its own `CLAUDE_CONFIG_DIR` for settings/skills.

Because the token is injected into the process environment at launch, a single
session that outlives its access token (roughly one hour) may need a relaunch to
pick up a fresh token; an environment variable cannot be updated inside a running
process. The background keepalive keeps the vault token fresh so the next launch
always starts clean.

## Install

Requires Go 1.22+. Builds on macOS, Linux, and Windows. clalt no longer uses the
macOS Keychain, so no code-signing setup is needed on any platform.

```
make install            # builds and copies to ~/bin (override INSTALL_DIR=...)
```

or just `make build` and copy `./clalt` somewhere on your PATH.

### Install on another device

One command from a clone of this repo (needs Go on that device):

```
./scripts/install.sh
```

It builds and installs to a PATH dir (`~/go/bin` if present, else
`~/.local/bin`). Re-runnable.

No Go on the target? Build portable binaries here and copy the matching one over:

```
make cross          # writes dist/clalt-{linux-amd64,linux-arm64,windows-amd64.exe,darwin-<arch>}
```

## Platform support

The account model, keychain-free `run`, usage, and the dashboard work on macOS,
Linux, and Windows. A couple of things differ by OS:

- How `run` hands the credential to claude: on macOS via
  `CLAUDE_CODE_OAUTH_TOKEN` (keychain-free, parallel); on Linux and Windows via an
  isolated per-profile `.credentials.json` scoped by `CLAUDE_CONFIG_DIR` (also
  parallel). Either way clalt never touches an OS keychain.
- Background keepalive: automatic. The first time you have an account, clalt
  registers a background job with the native scheduler for your OS (launchd on
  macOS, a systemd user timer on Linux, Task Scheduler on Windows) so tokens are
  refreshed ahead of expiry. See "Background keepalive" below.

## Commands

```
clalt list                 list accounts (active one marked with *)
clalt whoami               show the active account
clalt add <name>           capture the current Claude Code login as <name>
clalt login <name>         authenticate a new account via OAuth and store it
                           (--magic-link <url> to sign into someone else's
                           account first, see below)
clalt run <name> [-- ...]  run claude as <name>, keychain-free, in an isolated
                           profile (parallel: different accounts at once)
clalt budget [name ...]    show or set per-account spend caps (--daily/--weekly)
clalt sessions             list live parallel run sessions
clalt remove <name>        delete a slot (--force skips the confirm prompt)
clalt rename <old> <new>   rename a slot
clalt logout <name>        forget a slot's stored credential
clalt refresh <name>       refresh a slot's token now (--all for every account)
clalt keepalive            manage the background token-refresh job
clalt doctor [--fix]       diagnose account tokens and vault state
clalt export <file>        encrypted backup of all accounts
clalt import <file>        restore accounts from a backup
clalt usage [name] [--live]  usage stats (local by default)
clalt dashboard [--live]     interactive usage dashboard (TUI)
```

Add `--json` to most commands for machine-readable output.

### Typical flow

```
clalt login personal       # browser OAuth, stored in clalt's vault
clalt login work
clalt run personal         # launch claude as personal, keychain-free
clalt run work             # ... in another terminal, at the same time
```

`login` uses the manual-code OAuth flow: it opens your browser, you approve, then
paste the code shown on the callback page back into the terminal. You do not need
to be logged into Claude Code at all, and you never run `clalt switch` (there is
no such command) - every session runs through `clalt run`.

`clalt add <name>` is still available to capture an existing Claude Code login
from `~/.claude/.credentials.json` into the vault, but it is optional; `login`
covers the normal path.

### Logging into an account someone else owns

If you're adding an account that belongs to someone else, they can send you a
claude.ai magic link (their email sign-in link) instead of handing you a
password. Pass it straight to `login` and it opens first, in the same browser,
so that browser is signed into their account before the OAuth consent screen
comes up:

```
clalt login theiraccount --magic-link "https://claude.ai/...magic-link-url..."
```

clalt only accepts `https://claude.ai/...` or `https://*.anthropic.com/...`
links and refuses anything else - the link is a bearer credential for their
account, so treat it like a password: request it over a channel you trust, and
don't reuse or store it once you've logged in.

## Running accounts in parallel

`clalt run <name>` launches `claude` for one account without ever swapping a
global login, so you can use different accounts at the same time:

```bash
# terminal 1
clalt run work
# terminal 2, simultaneously
clalt run personal
```

- macOS: clalt injects `CLAUDE_CODE_OAUTH_TOKEN` for the account, so that child
  claude authenticates as that account regardless of any other login. Nothing
  goes through the Keychain.
- Linux and Windows: clalt stages the account's stored credential into
  `~/.config/clalt/profiles/<name>/.credentials.json` and points
  `CLAUDE_CONFIG_DIR` there. The profile rotates its own token; clalt syncs it
  back on exit.

On macOS, `clalt run` also supervises the session (see Caveats below for why):
it refreshes the account's token ahead of expiry and transparently restarts
claude with the fresh one, using `--resume` so the conversation carries over.
Pass `--no-supervise` to disable this and get a single unsupervised launch
instead.

Pass extra args to claude after `--` (e.g. `clalt run work -- --resume`).
`--refresh` re-resolves the token and re-copies your shared settings.

By default a profile inherits your full Claude config so a `run` session behaves
like your normal setup: every top-level directory under `~/.claude` (`skills`,
`plugins`, `commands`, `agents`, and any others) is symlinked, every top-level
settings file (`settings.json`, `settings.local.json`, `CLAUDE.md`, ...) is
copied, and your user-scope MCP servers from `~/.claude.json` are projected into
the profile's own `.claude.json`. Your `projects/` transcripts and per-session
state are never shared, so accounts stay separate. Sharing never overwrites a file
a profile already has (use `--refresh` to push updated settings) and never
clobbers a profile's own MCP server of the same name.

Pass `--isolated` for a clean-room profile that inherits none of your main config.
(The old `--link` flag is now the default and is accepted as a no-op.)

This relies on Claude Code's `CLAUDE_CODE_OAUTH_TOKEN` and `CLAUDE_CONFIG_DIR`,
both undocumented power-user variables that Anthropic could change.

macOS note: when `CLAUDE_CODE_OAUTH_TOKEN` is set, Claude Code may remove its own
Keychain login item on exit. That does not affect clalt (it does not use the
Keychain), but if you also run plain `claude` outside clalt on the same machine,
it may ask you to log in again. Logging back in, or just always launching through
`clalt run`, avoids that.

## Background keepalive

Claude rotates refresh tokens, and an unused account's token can lapse. clalt
keeps every account's vault token refreshed ahead of expiry with a small
background job that runs `clalt keepalive --once` on a schedule. Because `run`
reads the vault token at launch, this keeps back-to-back sessions starting fresh.

This is automatic: the first time you have an account (after `login`, `add`,
`run`, or `import`), clalt registers the job with your OS scheduler (launchd,
systemd user timer, or Task Scheduler) and prints a one-line note. You do not need
to run `keepalive install`.

Manage it:

```
clalt keepalive status              show whether the job is installed + token expiry
clalt keepalive install --dry-run   print exactly what would be scheduled, run nothing
clalt keepalive install             (re)install explicitly, e.g. with custom flags
clalt keepalive uninstall           remove it AND stop auto-enabling it
clalt keepalive --once              one refresh pass right now (no scheduler)
clalt keepalive                     foreground loop (ctrl-c stops)
```

`keepalive uninstall` records the opt-out, so clalt will not silently re-install
the job afterward; `keepalive install` clears that opt-out. To suppress
auto-enable everywhere without uninstalling, set `CLALT_NO_AUTO_KEEPALIVE=1`.
Tune the cadence with `--interval` (default 30m) and the early-refresh window
with `--buffer` (default 45m).

## Usage dashboard

`clalt dashboard` opens a full-screen interactive dashboard; `clalt usage` prints
the same data as static text (and is the fallback when stdout is not a terminal).
Both build stats from Claude Code's local transcripts
(`~/.claude/projects/**/*.jsonl`).

The dashboard has four tabs:

- overview: all accounts with color-graded 5h/7d utilization meters (green ->
  yellow -> red), reset countdowns (live) or token counts (local), and expiry
- tokens: today and lifetime totals, cache-hit rate, cost, top models,
  input/output/cache composition, hour-of-day histogram, and daily token and
  activity charts
- usage: per-account window detail and top projects
- sessions: live `clalt run` sessions

Controls: `left`/`right` (or `h`/`l`) switch tabs, `up`/`down` select account,
`enter` or `r` run the selected account (keychain-free), `d` delete it, `/` filter,
`:` command palette, `?` toggle the keybinding overlay, `q` quit. Actions are
performed after the dashboard closes.

Token accounting: the headline "tokens" figure is input + output only. Cache
reads are excluded from that number (with a high cache-hit rate they are ~30x
real input/output and would make totals meaningless) and are shown separately as
the composition breakdown and cache-hit rate. Cost still includes cache.

`--live` additionally queries Anthropic's usage endpoint for authoritative
window utilization. See the caveats below before relying on it.

## Environment overrides

The OAuth settings are reverse-engineered and can change with Claude Code
releases. Rather than force a recompile, each is overridable so you can react
immediately:

```
CLALT_OAUTH_CLIENT_ID    client id           (default 9d1c250a-...)
CLALT_AUTHORIZE_URL      authorize endpoint  (default https://claude.ai/oauth/authorize)
CLALT_TOKEN_URL          token endpoint      (default https://platform.claude.com/v1/oauth/token)
CLALT_REDIRECT_URI       redirect uri        (default https://platform.claude.com/oauth/code/callback)
CLALT_OAUTH_SCOPES       space-separated scopes
CLALT_API_BASE           host for --live usage + profile (default https://api.anthropic.com)
CLAUDE_CONFIG_DIR         Claude Code config dir (default ~/.claude)
XDG_CONFIG_HOME           clalt state dir parent (default ~/.config)
CLALT_NO_BROWSER         set to skip auto-opening the browser on login
CLALT_CLAUDE_BIN         override the claude binary `run` launches (default claude)
```

### Sandbox backend

`CLALT_KEYCHAIN_DIR` switches the sandbox secret store to a plaintext file-backed
store rooted at that directory. This exists only for tests and end-to-end runs so
the mutating flows can be exercised without touching real secrets. It is not the
production backend and does not encrypt at rest; do not point it at real secrets.

## Caveats (read these)

- Compliance: Anthropic's Feb 2026 Legal/Compliance update says OAuth tokens from
  Free/Pro/Max are intended exclusively for Claude Code and Claude.ai. clalt keeps
  the token in service of Claude Code itself. `--live` calls the usage endpoint
  from this tool with your token, which is a gray area; it is off by default and
  opt-in per invocation.
- Token in the environment: `clalt run` puts the access token in the child
  claude's environment (`CLAUDE_CODE_OAUTH_TOKEN`), which is readable by other
  processes running as the same user. It is never written to disk by clalt and
  never logged.
- Session length vs token life: Claude Code has no supported way to refresh
  CLAUDE_CODE_OAUTH_TOKEN inside an already-running process, so on macOS
  `clalt run` supervises the session instead - it refreshes the vault token
  ahead of expiry and transparently restarts claude with the fresh one and
  `--resume`, so a session survives past a single token's lifetime without
  you having to relaunch by hand. Pass `--no-supervise` to disable this and
  fall back to a single unsupervised launch (matches the old behavior: the
  session then hits an auth error once its token expires, and you relaunch
  through `clalt run` to start with a fresh one).
- The live usage endpoint is aggressively rate limited (persistent 429s with no
  Retry-After), so results are cached and polled sparingly; stale cache is shown
  when the endpoint is limiting.
- OAuth constants (client id, endpoints, redirect URI) are reverse-engineered from
  the Claude Code binary and can change between releases. If login or refresh
  breaks after a Claude Code update, override them via the environment variables
  above (no recompile) or update the defaults in `internal/oauth/oauth.go`.
- Email auto-labeling is best-effort; there is no confirmed public profile
  endpoint, so accounts fall back to the name you gave.
- Cost is an estimate from a static price table that will drift.

## Development

```
make check      # gofmt check, go vet, tests, build
```

Logic packages are unit-tested against an in-memory Keychain fake, so tests never
touch your real Keychain or Claude Code login.
