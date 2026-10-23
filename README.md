# clalt

Claude alt-account manager. Run several Claude accounts for Claude Code side by
side from one CLI: sign each account in, start sessions for any of them (in
parallel), and see a per-account usage dashboard. macOS and Linux; Windows is
best effort.

Unofficial community tool, not affiliated with Anthropic.

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

## Install

Requires Go 1.26.10 or newer to build, and claude 2.1.301 or newer to run.
Builds on macOS, Linux, and Windows.

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
clalt logout <name>        sign the account out, keep its history
clalt remove <name>        sign out and hide the account, history stays
                           (--force skips the confirmation)
clalt rename <old> <new>   change the label only, the folder never moves
clalt list                 accounts with their live sign-in state
clalt whoami               which account this shell is in
clalt doctor               check claude, the accounts and leftovers of v0.1.0
                           (only reports, never changes anything)
clalt migrate [--dry-run]  copy history from per-account profile folders into
                           the shared projects folder
clalt sessions             list live parallel run sessions
clalt budget [name]        show or set per-account spend caps
                           (--daily N, --weekly N, --clear)
clalt usage [name]         local usage stats
clalt dashboard            interactive usage dashboard (TUI)
clalt dashboard --demo     preview the new dashboard on demo data
clalt version              print build version (--json for machine-readable)
```

Add `--json` to most commands for machine-readable output. Accounts are named
by their label everywhere on the command line, ignoring case.

## Quick start

```
clalt login personal       # opens claude's sign-in in your browser
clalt login work
clalt run personal         # start claude as personal
clalt run work             # ... in another terminal, at the same time
clalt list                 # who is signed in, and until when
```

Everything after the account name goes to claude, with or without a `--` in
front: `clalt run work -- --resume` and `clalt run work --model opus` both work.
clalt exits with claude's own exit code.

## Documentation

The details are in `docs/`, one file per subject:

- [docs/commands.md](docs/commands.md): what each command does, how a sign-in is
  checked, what `run` does before it starts claude, listing and removing accounts
- [docs/sharing.md](docs/sharing.md): what is shared with `~/.claude`, the shared
  home, the history check and MCP servers
- [docs/auth.md](docs/auth.md): the environment variables clalt removes before
  every claude call
- [docs/doctor.md](docs/doctor.md): the checks of `clalt doctor`, and when it
  fails or only warns
- [docs/upgrading.md](docs/upgrading.md): upgrading from v0.1.0
- [docs/migrate.md](docs/migrate.md): `clalt migrate`, copying the history of old
  per-account profile folders into the shared history
- [docs/dashboard.md](docs/dashboard.md): usage, [budgets](docs/dashboard.md#budgets),
  [sessions](docs/dashboard.md#sessions) and the dashboard, with a preview of
  the new one
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
- clalt gives each claude call that is not interactive two minutes: `auth
  status` (with the version check it makes first), `--version` on its own, and
  `logout`, whose two minutes cover the version check, the sign-out and the
  status check together. At the deadline it interrupts claude, and kills it 30
  seconds later if it is still running (Windows cannot be sent an interrupt, so
  there the 30 seconds just run before the kill). It does not cancel sooner:
  real claude has no interrupt handler, so cancelling a call ends it at once,
  possibly in the middle of a token refresh, and that can sign the folder out.
  `claude auth login` is interactive and has no timeout, and neither has a
  session started by `clalt run`. A Ctrl-C at the terminal ends clalt but not the
  status check or sign-out it is waiting for: on macOS and Linux those run in a
  process group of their own, so the interrupt cannot end claude in the middle of
  a token refresh. The sign-in and the session stay in the foreground, since
  they read the terminal.
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
