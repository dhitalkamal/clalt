# clalt

Claude alt-account manager. Manage multiple Claude accounts for Claude Code from
one CLI: capture your current login, authenticate new accounts, switch the active
one in a keystroke, and see a per-account usage dashboard. macOS, Linux, and
Windows.

Unofficial community tool, not affiliated with Anthropic.

## How it works

Claude Code keeps a single active credential, and where it lives depends on the
OS:

- macOS: the Keychain entry `Claude Code-credentials`, mirrored to
  `~/.claude/.credentials.json`. On macOS Claude Code reads the Keychain.
- Linux and Windows: the file `~/.claude/.credentials.json` (or
  `$CLAUDE_CONFIG_DIR/.credentials.json`) under the `claudeAiOauth` key. There is
  no OS keychain in the loop.

clalt stores any number of named credential slots in its own local encrypted
vault (`~/.config/clalt/vault`, AES-256-GCM) and swaps the active one into
whichever location Claude Code reads on this OS. Non-secret metadata (email,
plan, timestamps) and the active pointer live in `~/.config/clalt/state.json`.
The vault is a file store on every platform, so listing and switching accounts
never prompt for an OS keychain password.

Claude rotates refresh tokens on every use, so a switch snapshots the currently
live credential back into the outgoing slot BEFORE loading the target. This is
why you should switch accounts through clalt rather than editing files: it keeps
the rotated token from being lost.

## Install

Requires Go 1.22+. Builds on macOS, Linux, and Windows; cgo (for the macOS
Keychain) is only used on macOS, so Linux and Windows builds are pure Go.

```
make install            # builds and copies to ~/bin (override INSTALL_DIR=...)
```

or just `make build` and copy `./clalt` somewhere on your PATH.

To stop macOS from prompting for your login password on every run, install with a
stable code-signing identity (one-time setup below):

```
make install CODESIGN_IDENTITY=clalt-codesign
```

Do not install clalt with `go install`: that path skips the Makefile signing step
and leaves the binary ad-hoc signed, so the prompts come back. Use `make install`
(point it at your Go bin with `INSTALL_DIR=$HOME/go/bin` if that is where your
`clalt` lives). See "macOS Keychain prompts" below. (Linux and Windows do not
need signing; that whole concern is macOS-only.)

### Install on another device

One command from a clone of this repo (needs Go on that device):

```
./scripts/install.sh
```

It builds, installs to a PATH dir (`~/go/bin` if present, else `~/.local/bin`),
and on macOS signs with `clalt-codesign` if that identity exists (set
`CODESIGN_IDENTITY=<name>` otherwise). Re-runnable.

No Go on the target? Build portable binaries here and copy the matching one over:

```
make cross          # writes dist/clalt-{linux-amd64,linux-arm64,windows-amd64.exe,darwin-<arch>}
```

Linux/Windows binaries are self-contained (pure Go) - drop the file on the
target's PATH and run. For another Mac, build from source there (`install.sh`) so
the code-signing identity is local to that machine.

## Platform support

The account model, switching, parallel `run`, usage, and the dashboard work on
macOS, Linux, and Windows. A few things differ by OS:

- Credential storage: on macOS the live credential goes through the Keychain
  (plus the file); on Linux and Windows it is the `~/.claude/.credentials.json`
  file alone, matching what Claude Code itself reads on each OS.
- Parallel `clalt run`: truly isolates accounts on Linux and Windows (Claude Code
  scopes credentials per `CLAUDE_CONFIG_DIR` there). On macOS it cannot isolate
  because Claude Code reads one global Keychain item regardless; use
  `clalt switch` on macOS. See the note under "Switch vs run".
- Background keepalive: automatic. The first time you have an account, clalt
  registers a background job with the native scheduler for your OS - launchd on
  macOS, a systemd user timer on Linux, Task Scheduler on Windows - so tokens are
  refreshed ahead of expiry without you doing anything. See "Background keepalive"
  below to preview, disable, or manage it.
- Keychain password prompts and code-signing: a macOS-only concern.

## Commands

```
clalt list                 list accounts (active one marked with *)
clalt whoami               show the active account
clalt add <name>           capture the current Claude Code login as <name>
clalt login <name>         authenticate a new account via OAuth and store it
clalt switch [name]        switch active account (no name opens a picker)
clalt run <name> [-- ...]  run claude as <name> in parallel (isolated profile)
clalt next [--live]        rotate to the next account in the fallback chain
clalt fallback [names...]  show or set the rotation chain (--threshold N)
clalt remove <name>        delete a slot (--force skips the confirm prompt)
clalt rename <old> <new>   rename a slot
clalt logout <name>        forget a slot's stored credential
clalt refresh <name>       refresh a slot's token now
clalt doctor [--fix]       diagnose keychain/file sync and token expiry
clalt export <file>        encrypted backup of all accounts
clalt import <file>        restore accounts from a backup
clalt usage [name] [--live]  usage stats (local by default)
clalt dashboard [--live]     interactive usage dashboard (TUI)
```

Add `--json` to most commands for machine-readable output.

### Typical flow

```
clalt add work             # you are already logged in as work; capture it
clalt login personal       # browser OAuth for a second account
clalt switch personal      # now Claude Code uses personal
clalt switch work          # ... and back
```

`login` uses the manual-code OAuth flow: it opens your browser, you approve, then
paste the code shown on the callback page back into the terminal.

### Standalone: no main Claude Code account needed

You do not need to be logged into Claude Code at all, or ever run `clalt add`.
Add accounts with OAuth and use them straight through isolated `run` sessions:

```
clalt login personal       # OAuth, stored in clalt's vault
clalt login work
clalt run personal         # launches claude as personal, no global login touched
```

`clalt run` stages the account's own credential into an isolated profile and
launches `claude` there, so nothing depends on a global `~/.claude` login. You
still need the `claude` binary installed (clalt drives it); you just do not need a
"main" account. `switch` remains available if you also want to swap the global
login, but it is optional.

macOS note: because Claude Code on macOS reads one global Keychain item and
ignores per-profile credentials, the isolated `run` path above works on Linux and
Windows. On macOS the no-main equivalent is `clalt login <name>` then
`clalt switch <name>` - clalt writes the login for you, so you still never have to
log into Claude Code by hand, but only one account is active at a time.

## Switch vs run: one at a time, or many at once

clalt supports two models, because they solve different problems:

- `clalt switch <name>` swaps the single global Claude Code login. Simple, but
  only one account is active machine-wide at a time; switching affects every
  Claude Code session.
- `clalt run <name>` launches `claude` in an isolated per-account config dir
  (`CLAUDE_CONFIG_DIR`), so you can use different accounts **at the same time**.
  Run it in two terminals with two accounts and both are live in parallel; your
  normal `claude` on the global login keeps working untouched.

```bash
# terminal 1
clalt run work
# terminal 2, simultaneously
clalt run personal
```

How `run` works depends on where Claude Code reads its credential on your OS:

- Linux and Windows: it stages the account's stored credential into
  `~/.config/clalt/profiles/<name>/.credentials.json` and points
  `CLAUDE_CONFIG_DIR` there, so each account is fully isolated and can run in
  parallel. The profile rotates its own token; clalt syncs it back on exit.
- macOS: Claude Code reads the one global Keychain item and ignores the
  per-profile file, so `run` loads the account into the live login (the same
  place `switch` writes) before launching, and captures any rotation back on
  exit. This means `run <name>` on macOS just works without a fresh login, but
  accounts are not parallel there - they share the single login (clalt warns if
  you start a second run).

`CLAUDE_CONFIG_DIR` still isolates settings/skills on every OS. Pass extra args to
claude after `--` (e.g. `clalt run work -- --resume`). `--refresh` re-stages the
credential and re-copies your shared settings.

By default a profile inherits your full Claude config so a `run` session behaves
like your normal setup: every top-level directory under `~/.claude` (`skills`,
`plugins`, `commands`, `agents`, and any others) is symlinked, every top-level
settings file (`settings.json`, `settings.local.json`, `CLAUDE.md`, ...) is
copied, and your user-scope MCP servers from `~/.claude.json` are projected into
the profile's own `.claude.json`. Your live credential, `projects/` transcripts,
and per-session state are never shared, so accounts stay separate. Sharing never
overwrites a file a profile already has (use `--refresh` to push updated settings)
and never clobbers a profile's own MCP server of the same name.

Pass `--isolated` for a clean-room profile that inherits none of your main config.
(The old `--link` flag is now the default and is accepted as a no-op.)

This relies on Claude Code's `CLAUDE_CONFIG_DIR`, an undocumented power-user
variable that Anthropic could change.

Known macOS limitation: on macOS Claude Code always reads the single global
Keychain item `Claude Code-credentials`; `CLAUDE_CONFIG_DIR` scopes credentials to
a per-dir `.credentials.json` only on Linux and Windows. clalt handles this so
`run <name>` still works without a re-login (it loads the account into the live
login first), but two accounts cannot be live at once on macOS - `run` and
`switch` both drive the one shared login there. True parallel accounts work on
Linux/Windows. This may change if Anthropic adds macOS support for per-dir
credentials.

## macOS Keychain prompts

If macOS keeps asking for your login password when you run clalt (or even when you
run plain `claude`), the cause is code signing. Claude Code's live credential
lives in the macOS Keychain, and macOS grants an app access to a Keychain item by
its code-signing identity. A plain `go build` (or `go install`) produces an
ad-hoc signature whose code hash changes on every rebuild, so a Keychain "always
allow" grant never sticks and you are re-prompted each run. clalt's own per-account
secrets avoid this by living in a local encrypted vault, but the shared Claude Code
credential must go through the Keychain, so clalt needs a stable identity.

Fix it once by signing clalt with a stable self-signed code-signing identity.

1. Create the identity (one time). In Keychain Access: menu
   Certificate Assistant -> Create a Certificate. Name it `clalt-codesign`,
   Identity Type "Self Signed Root", Certificate Type "Code Signing", then create.
   This is a local trust decision; the cert never leaves your machine and is not
   for distribution.

2. Build and install signed with it:

   ```
   make install CODESIGN_IDENTITY=clalt-codesign
   ```

   (add `INSTALL_DIR=$HOME/go/bin` if your `clalt` lives there). Confirm the
   binary is stably signed:

   ```
   clalt doctor        # "code signing:" should read "stable", not "adhoc"
   ```

3. First run after signing, macOS prompts once for the Keychain item; click
   "Always Allow". Because the identity is now stable, that grant persists across
   future rebuilds and the prompts stop.

clalt only ever updates the Claude Code Keychain item in place, never deletes and
recreates it, so the access grant Claude Code set for its own binary is preserved
and `claude` itself is not re-prompted either. If `claude` was already prompting
before you first signed clalt, click "Always Allow" once for it too.

`clalt doctor` reports the running binary's signing state under "code signing" and
prints the fix steps when it detects an ad-hoc or unsigned build.

## Background keepalive

Claude rotates refresh tokens, and an unused account's token can lapse. clalt
keeps every account's token refreshed ahead of expiry with a small background
job that runs `clalt keepalive --once` on a schedule.

This is automatic: the first time you have an account (after `login`, `add`,
`switch`, `run`, or `import`), clalt registers the job with your OS scheduler
(launchd, systemd user timer, or Task Scheduler) and prints a one-line note. You
do not need to run `keepalive install`.

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

## Fallback rotation

When one account nears its limit, rotate to the next without hunting for a free
one:

```bash
clalt fallback work personal spare      # define the rotation order
clalt fallback --threshold 0.85         # rotate off an account at 85% (with --live)
clalt next                              # move to the next account in the chain
clalt next --live                       # skip accounts already near their 5h limit
```

`clalt next` (no flag) simply advances to the next account in the chain (run it
when you hit a wall). `clalt next --live` reads each account's authoritative 5h
utilization and picks the first one below the threshold, so it skips accounts that
are already spent. With no chain configured it uses all accounts in name order.
The chain and each account's utilization also show at the bottom of the dashboard
overview.

## Usage dashboard

`clalt dashboard` opens a full-screen interactive dashboard; `clalt usage` prints
the same data as static text (and is the fallback when stdout is not a terminal).
Both build stats from Claude Code's local transcripts
(`~/.claude/projects/**/*.jsonl`) and attribute activity to whichever account was
active at the time, per clalt's switch log.

The dashboard has four tabs:

- overview: all accounts with color-graded 5h/7d utilization meters (green ->
  yellow -> red), reset countdowns (live) or token counts (local), and expiry
- tokens: today and lifetime totals, cache-hit rate, cost, top models,
  input/output/cache composition, hour-of-day histogram, and daily token and
  activity charts
- usage: per-account window detail and top projects
- fallback: the rotation chain with per-account utilization and the policy

Controls: `left`/`right` (or `h`/`l`) switch tabs, `up`/`down` select account,
`enter` switch to the selected account, `r` run it in parallel, `n` rotate to the
next account, `?` toggle the keybinding overlay, `q` quit. Actions are performed
after the dashboard closes.

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
```

### Sandbox backend

`CLALT_KEYCHAIN_DIR` switches the secret store from the macOS Keychain to a
plaintext file-backed store rooted at that directory. This exists only for tests
and end-to-end runs so the mutating flows (add/switch/login) can be exercised
without touching the real Keychain or your real Claude Code login. It is not the
production backend and does not encrypt at rest; do not point it at real secrets.

## Caveats (read these)

- Compliance: Anthropic's Feb 2026 Legal/Compliance update says OAuth tokens from
  Free/Pro/Max are intended exclusively for Claude Code and Claude.ai. Credential
  swapping keeps the token in service of Claude Code itself. `--live` calls the
  usage endpoint from this tool with your token, which is a gray area; it is off
  by default and opt-in per invocation.
- The live usage endpoint is aggressively rate limited (persistent 429s with no
  Retry-After), so results are cached and polled sparingly; stale cache is shown
  when the endpoint is limiting.
- OAuth constants (client id, endpoints, redirect URI) are reverse-engineered from
  the Claude Code binary and can change between releases. If login or refresh
  breaks after a Claude Code update, override them via the environment variables
  above (no recompile) or update the defaults in `internal/oauth/oauth.go`.
- Email auto-labeling is best-effort; there is no confirmed public profile
  endpoint, so accounts fall back to the name you gave.
- Local usage is per-machine and per-account-accurate only for activity after
  clalt began managing switches. Cost is an estimate from a static price table
  that will drift.
- Switching while a `claude` process is running takes effect on that session's
  next start/refresh, not instantly.

## Development

```
make check      # gofmt check, go vet, tests, build
```

Logic packages are unit-tested against an in-memory Keychain fake, so tests never
touch your real Keychain or Claude Code login.
