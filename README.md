# clalt

Claude alt-account manager. Run several Claude accounts for Claude Code side by
side from one CLI: sign each account in, start sessions for any of them (in
parallel), and see a per-account usage dashboard. macOS and Linux; Windows is
best effort.

Unofficial community tool, not affiliated with Anthropic.

## Getting started

You need claude 2.1.301 or newer on your PATH, and Go 1.26.10 or newer to build
clalt.

### 1. Install

```
go install github.com/dhitalkamal/clalt@latest   # puts clalt in ~/go/bin
```

or, from a clone of this repo:

```
make install            # builds and copies to ~/bin (override INSTALL_DIR=...)
```

`make install` writes to `~/bin`, which is not on every PATH. If `clalt: command
not found`, add it (`export PATH="$HOME/bin:$PATH"`) or set `INSTALL_DIR`. The
same goes for `~/go/bin` with `go install`. Check with `clalt version`.

### 2. Add your accounts

```
clalt login personal       # opens claude's own sign-in in your browser
clalt login work           # a new name makes a new account
clalt list                 # who is signed in, and until when
```

If your browser is already signed in to another claude.ai account, use a
private window for the sign-in. clalt refuses a sign-in whose email already
belongs to another account.

### 3. Run sessions, switch accounts

```
clalt run personal         # start claude as personal
clalt run work             # in another terminal, at the same time
clalt run                  # let clalt pick the account with the most room
clalt whoami               # which account this shell is in
```

There is no global "switch". Each `clalt run <name>` is its own session, so any
number of accounts run in parallel. Everything after the name goes to claude:
`clalt run work --model opus`. All accounts share one conversation history, so
`clalt resume` can continue a conversation under another account. To move a
running session, use `clalt move <session> <account>` (see
[docs/move.md](docs/move.md)).

### 4. Dashboard

```
clalt dashboard            # accounts, sessions, limits, usage and budgets
clalt dashboard --demo     # the same on made-up data, touches no account
```

Keys are listed in [docs/dashboard.md](docs/dashboard.md).

### 5. Office

```
clalt office               # live 3D garden campus of your agents, in the browser
clalt office on            # same, in the background (also off, open, status)
```

It prints a link with a one-run token and opens it. Press Ctrl-C to stop it, or
use `on` and `off` to run it with no terminal open. Pick the default template in
the page's settings. Build mode (change the floor plan) opens from the page.

Templates: `hall` (the default), `rows`, `village`, `river` and `street`. Pick
one for a run with `--template <id>`, or set the default in the page's Settings.

Navigation on the page (the sky camera, the default):

```
W A S D, arrows   move over the campus      Q E        turn
R, C              higher, lower             wheel      zoom to the cursor
left drag         grab the ground           right drag turn and tilt
double-click      fly to that spot          Home       back to the overview
G                 ground level: walk        V          first or third person
F                 follow the agent under the cursor or selected
Esc               leave ground or follow, or go to the overview
1-5               templates
P                 auto spin                 L          labels
```

More in [docs/office.md](docs/office.md).

### 6. Optional: the feed for plain claude

`clalt run` sessions feed the dashboard and the office on their own. To include
sessions you start with plain `claude` (config in `~/.claude`), run:

```
clalt feed install         # asks first, --yes skips the question
clalt feed status
```

It needs `~/.claude` to be signed in as the same email as one of your accounts.
It copies `~/.claude/settings.json` to `~/.config/clalt/backups/settings/`,
then adds clalt's hooks and status line to it, after your own. Your hooks and
status line keep running. It does not change anything else, and it is refused
on Windows. Details in [docs/feed.md](docs/feed.md).

### Uninstall

```
clalt feed uninstall       # removes exactly what feed install added
clalt remove <name> --delete   # per account: signs out, deletes its folder
rm ~/bin/clalt             # or ~/go/bin/clalt
rm -r ~/.config/clalt      # clalt's own state, after the steps above
```

Sign accounts out first (`remove --delete` does it). On macOS the login lives
in the Keychain, and deleting the folder alone leaves that entry behind. Your
history in `~/.claude` is never touched.

## Terms and data

- clalt only reads the files Claude Code keeps on your machine: its transcripts
  (for usage), its session hooks and `claude auth status`. It stores no prompts,
  no replies and no tokens, and never reads a credential file or the keychain.
- The office server listens on 127.0.0.1 only, with a new random token per run.
  The page loads nothing from the internet. Anyone who has the link while it
  runs can look at it, so do not paste it in a chat.
- Nothing is sent anywhere by clalt itself. Each account is yours, and each
  account's terms with Anthropic apply. Auto-switch is meant for accounts you
  are entitled to use for the work at hand.
- clalt is unofficial and not affiliated with Anthropic. It is released under
  the license in [LICENSE](LICENSE), with no warranty.

## How it works

Each account is a folder, `~/.config/clalt/accounts/<id>`, that clalt hands to
claude as `CLAUDE_CONFIG_DIR`. claude keys that account's login to the path of
the folder, so the folder never moves. Where the login is kept depends on the
system: on macOS it is an entry in the Keychain, named after the folder's path,
and only on Linux and Windows is it a file (`.credentials.json`) in the folder.
clalt stores no tokens and reads none: it asks claude who is signed in
(`claude auth status`), and signs accounts in and out through claude's own
`auth login` and `auth logout`. What clalt itself remembers lives in
`~/.config/clalt/state.json`: each account's label, the email that signed in,
plan, organization and when, last use, and spend caps.

The folder is linked to `~/.claude`, so every account sees the same settings,
skills, plugins, commands, agents, CLAUDE.md and, importantly, the same
conversation history. Only the login (and a few pieces of per-account state)
stay separate.

## More ways to install

`./scripts/install.sh` builds and installs to `~/go/bin` if present, else
`~/.local/bin` (needs Go). No Go on the target? `make cross` writes portable
binaries to `dist/clalt-{linux-amd64,linux-arm64,windows-amd64.exe,darwin-<arch>}`;
copy the matching one over.

## Platform support

clalt calls the `claude` binary, so it works wherever claude does. Windows is
best effort: linking the shared claude home needs symlinks, which Windows only
allows in Developer Mode. A link that cannot be made is reported by name and
the rest still work.


## Commands

```
clalt login <name>         sign an account in through claude's own login. a new
                           name makes the account, a removed one comes back
clalt run <name> [-- ...]  run claude as <name>, in its own folder, with the
                           history shared with ~/.claude (accounts run in parallel)
clalt run [-- ...]         run claude as the allowed account with the most room
                           for this folder
clalt rules <name>         the folders an account may serve, and auto-switch
                           (--add-folder, --remove-folder, --auto-switch,
                           --no-auto-switch)
clalt move <session> <account>
                           move a running session at its next pause
clalt logout <name>        sign the account out, keep its history
clalt remove <name>        sign out and hide the account, history stays
                           (--force skips the confirmation)
clalt remove <name> --delete
                           sign out, delete its folder and record
clalt rename <old> <new>   change the label only, the folder never moves
clalt list                 accounts with their live sign-in state
clalt whoami               which account this shell is in
clalt doctor               check claude, the accounts and leftovers of v0.1.0
                           (only reports, never changes anything)
clalt migrate [--dry-run]  copy history from per-account profile folders into
                           the shared projects folder
clalt sessions             list live parallel run sessions
clalt resume [words]       list conversations of every account, or continue one
                           with `clalt resume <id> [--account <name>]`
clalt launch <name>        run claude as <name> in a new kitty tab or tmux window
clalt jump <session>       bring the terminal of a running session to the front
clalt notify [on|off|status|test]
                           desktop alerts when a session needs you
clalt feed install|uninstall|status
                           the live feed for plain claude (forget-default and
                           remember-default control usage attribution)
clalt budget [name]        show or set per-account spend caps
                           (--daily N, --weekly N, --clear)
clalt usage [name]         local usage stats
clalt dashboard            full-screen dashboard of accounts, sessions and usage
clalt dashboard --demo     the dashboard on made-up data, touches no account
clalt office               live 3D garden campus of your agents in the browser
                           (--port N, --no-open, --template rows|village|river|street|hall)
clalt office on|off|open|status [--link]|autostart on|off|status
                           run the office in the background, start it at login
clalt office layout list|use <name>|export <name> [file]|import <file>|reset
                           the office floor plans
clalt mcp                  read-only MCP server for Claude Code (docs/mcp.md)
clalt version              print build version (--json for machine-readable)
```

Add `--json` to most commands for machine-readable output. Accounts are named
by their label everywhere on the command line, ignoring case. Everything after
the account name of `clalt run` goes to claude, with or without a `--`, and
clalt exits with claude's own exit code.

## Documentation

The details are in `docs/`, one file per subject:

- [docs/commands.md](docs/commands.md): what each command does, how a sign-in is
  checked, what `run` does before it starts claude, listing and removing accounts
- [docs/sharing.md](docs/sharing.md): what is shared with `~/.claude`, the shared
  home, the history check and MCP servers
- [docs/auth.md](docs/auth.md): the environment variables clalt removes before
  every claude call
- [docs/move.md](docs/move.md): account rules, how `clalt run` picks an account,
  moving a session between accounts, auto-switch and its terms
- [docs/feed.md](docs/feed.md): the session registry and feed that hooks and the
  status line fill, and what `clalt run` passes to claude for it
- [docs/doctor.md](docs/doctor.md): the checks of `clalt doctor`, and when it
  fails or only warns
- [docs/upgrading.md](docs/upgrading.md): upgrading from v0.1.0
- [docs/migrate.md](docs/migrate.md): `clalt migrate`, copying the history of old
  per-account profile folders into the shared history
- [docs/dashboard.md](docs/dashboard.md): usage, [budgets](docs/dashboard.md#budgets),
  [sessions](docs/dashboard.md#sessions) and the dashboard, with its keys
- [docs/office.md](docs/office.md): `clalt office`, the live 3D office of your
  agents, its controls and what it shows
- [docs/extras.md](docs/extras.md): `clalt resume`, `clalt launch`, `clalt jump`
  and the needs-you alerts
- [CHANGELOG.md](CHANGELOG.md): what changed in each release

## Environment overrides

```
CLAUDE_CONFIG_DIR   the shared claude home, when it points outside clalt's own
                    folder (default ~/.claude)
XDG_CONFIG_HOME     clalt state dir parent (default ~/.config)
CLALT_CLAUDE_BIN    the claude binary clalt runs (default claude, from PATH)
```

## Caveats (read these)

- clalt relies on claude's `auth login`, `auth status` and `auth logout`
  commands and on `CLAUDE_CONFIG_DIR`. Anthropic could change them, and the
  `auth status` json keys are parsed as documented at the time of writing
  (unknown keys are ignored, output that is not json is an error). It needs
  claude 2.1.301 or newer.
- A login belongs to the folder it was made in. Do not move or rename an
  account folder; `clalt rename` changes only the label for that reason.
- Which files claude saves by replacing them (which breaks a link) is not
  verified. clalt keeps such a file and reports it, so nothing is lost, but that
  account then has its own copy until you resolve it.
- MCP sync writes an account's `.claude.json` while another session of the same
  account may be running. The write is atomic and only happens when something
  changed, but a small race is accepted.
- Non-interactive claude calls get two minutes, then an interrupt and a kill
  30 seconds later. See [docs/commands.md](docs/commands.md#timeouts).
- Text that claude reports (the email, plan and organization, and its error
  lines) and file names found on disk have control characters, line separators
  and bidirectional overrides shown as escapes (`\x1b`) before they are stored
  or printed, so an organization name cannot put escape sequences on your
  terminal. The zero width joiners that scripts and emoji need are kept.
- Cost is an estimate from a static price table that will drift.

## Development

```
make check      # gofmt check, go vet, tests, build
```

Tests run against a temp HOME and temp state dirs and a fake claude (a small
shell script that clalt is pointed at with `CLALT_CLAUDE_BIN`), so they never
read or write your real `~/.claude`, `~/.claude.json` or `~/.config/clalt`, and
never run the real claude.
