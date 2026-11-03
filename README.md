# clalt

Keep your Claude accounts apart in Claude Code, for example a personal account
and your work account. Sign each one in, start sessions for any of them (in
parallel) from one CLI, and see usage per account in a dashboard and a live 3D
office of your agents. macOS and Linux; Windows is best effort.

Unofficial community tool, not affiliated with Anthropic.

## Getting started

You need claude 2.1.301 or newer on your PATH, and Go 1.26.10 or newer to build
clalt.

### 1. Install

Download the archive for your system from the
[releases page](https://github.com/dhitalkamal/clalt/releases) and put `clalt`
on your PATH. Check one with `gh attestation verify <archive> --repo
dhitalkamal/clalt` (or against `checksums.txt`).

With Go:

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
clalt run                  # pick one of the accounts allowed in this folder
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

More in [docs/office.md](docs/office.md). To open the office from your phone or
another computer, see [docs/office-remote.md](docs/office-remote.md).

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

### 7. Optional: the Claude Code plugin

`clalt mcp` is a read-only MCP server, so Claude can tell you your usage per
account, your running sessions and what you did today. Install it as a plugin
from inside Claude Code:

```
/plugin marketplace add dhitalkamal/clalt
/plugin install clalt@clalt
```

Details in [docs/mcp.md](docs/mcp.md).

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
- Nothing is sent anywhere by clalt itself. The one exception is opt-in: with
  remote access set up ([docs/office-remote.md](docs/office-remote.md)), the
  office fetches Cloudflare's public signing keys to check logins.
- clalt runs the official claude binary, unmodified. Every sign-in happens in
  claude's own login flow, and clalt never sees a token.

### Use it within Anthropic's terms

- Every account you add must be your own, and each one stays under its own
  terms: the [Consumer Terms](https://www.anthropic.com/legal/consumer-terms)
  for Free, Pro and Max, the
  [Commercial Terms](https://www.anthropic.com/legal/commercial-terms) for Team,
  Enterprise and the API. Do not share an account or sign in to someone else's.
- clalt is for keeping separate accounts apart, like a personal account and a
  work account. It is not a way to combine subscriptions to get more usage than
  your plans give you. Anthropic's
  [Usage Policy](https://www.anthropic.com/legal/aup) does not allow bypassing
  restrictions in its products, and plan limits assume ordinary, individual
  use ([Claude Code legal and compliance](https://code.claude.com/docs/en/legal-and-compliance)).
- Folder rules, `clalt move` and auto-switch (off by default) are there so the
  right account serves the right work, for example work repos on the work
  account. You are responsible for how you use them.
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

Every command with its flags is in [docs/commands.md](docs/commands.md#command-list).
Add `--json` to most commands for machine-readable output.

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
- [docs/office-idle.md](docs/office-idle.md): what idle agents do, and how to choose
- [docs/office-remote.md](docs/office-remote.md): opening the office from another
  device through a Cloudflare Tunnel
- [docs/office-versions.md](docs/office-versions.md): how two office tabs save
  without overwriting each other
- [docs/mcp.md](docs/mcp.md): `clalt mcp` and the Claude Code plugin
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

## Contributing and security

Bug reports and pull requests are welcome ([CONTRIBUTING.md](CONTRIBUTING.md)).
Report security problems privately ([SECURITY.md](SECURITY.md)).

## Development

```
make check      # gofmt check, go vet, tests, build
```

Tests run against a temp HOME and temp state dirs and a fake claude (a small
shell script that clalt is pointed at with `CLALT_CLAUDE_BIN`), so they never
read or write your real `~/.claude`, `~/.claude.json` or `~/.config/clalt`, and
never run the real claude.
