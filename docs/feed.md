# The session feed

`clalt run` keeps a record of each session it starts, and claude tells clalt what
the session is doing. Nothing here changes your own claude settings: the hooks and
the status line are given to claude with `--settings` for that session only.
Plain `claude` (the default home, `~/.claude`) is covered by the opt-in
`clalt feed install` further down.

## What `clalt run` adds

- A handle (16 hex characters) in `CLALT_SESSION`, which claude's hooks and status
  line inherit.
- A record in `~/.config/clalt/sessions/<handle>.json`: account, folder, the
  `clalt run` process, claude's process, the claude session id (filled by the first
  hook or status line call, and updated after `/clear` or `/resume`), the folder it
  was started in, the start time and where the terminal is (kitty, tmux, an IDE or
  a plain terminal, from the environment). It is written before claude starts and
  removed when claude ends. `clalt sessions --json` shows these fields.
- `--settings <json>` in front of claude's arguments, installing hooks and a
  status line that run `clalt hook <event>` and `clalt statusline` (the path of
  the running clalt, quoted). Claude merges hooks with yours, so yours keep
  running. If you pass your own `--settings`, clalt does not add its own and says
  that the feed is off for that session.

## The feed

`~/.config/clalt/feed/sessions/<handle>.json` holds `state` (`starting`, `busy`,
`idle`, `needs-you` or `ended`), `since`, `reason` (the notification type while
`needs-you`), `lastError` (for example `rate_limit` when a turn ended on an API
error), `model`, `cwd`, `contextPct` and `updatedAt`.
`~/.config/clalt/feed/accounts/<id>.json` holds the 5-hour and 7-day used
percentage with the time each resets, and when it was seen. Files that are
missing or corrupt count as "not there". The details and the state machine are in
[plans/session-feed.md](plans/session-feed.md).

Limits come from the status line, which claude only fills for claude.ai Pro and Max
accounts and after the first reply of a session. Several sessions of one account
report: for each window the later reset wins, and inside a window the higher use.

Known gaps: an interrupt (Esc) fires no hook, so `busy` stays until the next
prompt; `needs-you` ends at the next tool result, and a permission notification
that arrives within 6 seconds of a tool result is ignored as late (a denied prompt
or an interrupt can still leave it until the next event); in an interactive session claude
holds hooks and the status line back until you have accepted the trust dialog of
the folder, so the session stays `starting` until then.

## The status line

With no status line of your own, clalt's prints nothing, and claude hides most of its
footer hints while any status line is set. With one in `~/.claude/settings.json`,
`clalt statusline` runs your command with the same input and prints its output
unchanged (3 second limit, your exit code is passed on). Only the settings file of
your claude home is read, never a project's: a project's command waits for the
trust dialog and clalt would run it without. A project that sets its own status
line loses it in a `clalt run` session.

## Errors

The hooks never print and always exit 0. What goes wrong is written to
`~/.config/clalt/logs/hook.log`, and `clalt doctor` reports the errors of the last
24 hours, sessions that never reported, and records of sessions that are gone.

## Usage by session

Each time a hook or the status line sees a new claude session id under a clalt run
session, it appends the id and the account to `~/.config/clalt/session-accounts.jsonl`
(this file is kept, unlike the feed). `clalt usage` and `clalt budget` use it for the
usage from the first sign-in of the folder design on: a transcript record goes to the
account its claude session ran under, and a conversation that moved is split by time.
Sessions that were not started by `clalt run` are attributed to the account of
the default home when there is one, see below, and stay unattributed otherwise.

## Plain claude: the default home

People run plain `claude` (config in `~/.claude`, its own login) next to
`clalt run`. clalt sees those sessions only when it can tell whose they are, and
it does that by email.

**Matching.** clalt asks claude about the default home exactly as plain claude
sees it (`CLAUDE_CONFIG_DIR` unset, no key or token variables, no short timeout)
and keeps the answer in `~/.config/clalt/default-home.json`. If it is a claude.ai
login whose email equals the recorded email of one visible account (case does
not matter), that account is the default home's. A different email, a login that
is not claude.ai (an API key, a third party provider) or two accounts with the
same email leave plain claude outside clalt, and `clalt doctor` says why. When
claude cannot be asked, an earlier match is kept and the reason is recorded.
`clalt list` and the dashboard Profile say "also the default claude (~/.claude)".
Asking claude is slow, so only `clalt feed status`, `clalt feed install`,
`clalt usage` and the dashboard do it, at most once a day (feed status always).
Hooks, the status line, doctor and the other views read the file.

**Usage.** A session that no clalt run session claimed is attributed to the
matched account when it started at or after the later of the first sign-in of the
folder design and the moment this match began (`matchedSince` in
`default-home.json`, reset whenever the account or the state changes). So a
`/login` as another account, or plain claude going from outside to matched, never
charges the new account for what ran before; when the match changes, the sessions
the old one earned are written to the ledger first.
Each one is written to `session-accounts.jsonl` with `"source":"default"`, so it
can be seen and undone: `clalt feed forget-default` removes those lines (usage is
unattributed again) and stops clalt from adding them,
`clalt feed remember-default` turns it back on. A session that already has a line
is never moved.

**Live feed, opt in.**

```
clalt feed install [--yes]     add clalt's hooks and status line to ~/.claude/settings.json
clalt feed uninstall [--yes]   take exactly those out again
clalt feed status [--json]     installed or not, the account, the backup
```

A plain claude session that starts while the cached answer is older than 5 minutes
makes its hook ask claude again first; if that does not finish inside the hook's
time, the session is not recorded or attributed at all rather than guessed.
(The status line never asks: it waits for the hook.)

Install is refused on Windows: claude runs hook commands in Git Bash, or in
PowerShell when Git Bash is missing, and one command cannot be made reliable for
both. `clalt run` sessions still feed clalt there.

Install needs a match, asks first (`--yes` for scripts; with no terminal and no
`--yes` it refuses) and refuses when `settings.json` is not a regular file (after
following a link) or not a JSON object. Before writing it copies the file to
`~/.config/clalt/backups/settings/settings-<UTC time>.json`. It adds one handler
per event of the list above, after yours (your hooks keep running), and every
command ends in the word `clalt-feed`, which is how uninstall and status find them.
Nothing else in the file is touched. If you have a `statusLine` of your own, clalt
keeps it in `~/.config/clalt/feed-install.json`, not in `settings.json`, puts its
own in its place (with your `padding`, `refreshInterval` and
`hideVimModeIndicator`) and `clalt statusline` runs yours. The file is written
atomically, after checking that the file is still what the question was about;
if claude saved it meanwhile, install stops with "settings.json changed while
waiting, run clalt feed install again" and writes nothing. A second install
changes nothing.

**Symlinked settings.json.** When `settings.json` is a link (a dotfiles repo, say),
install and uninstall edit the file it points to and leave the link alone. clalt
resolves the link first and refuses a dangling link, a link loop, a target that is
not a regular file, one owned by another user (not checked on Windows) and one
inside `~/.config/clalt`. Before asking it prints the real path, and when that file
is in a git work tree it warns that the change shows up in that repo and that the
hooks use this machine's clalt path, so they are silent no-ops where that path does
not exist. `--yes` skips the question. The write is atomic in the target's own
folder (temp file, then rename), keeps the target's mode, and the backup is the
target's old bytes. The resolved target is recorded in `feed-install.json`;
uninstall edits that same file and refuses if the link points elsewhere now.
`clalt feed status` never fails on a link: it prints "settings.json is a link to
<target>" with the install state (`settingsLinkTarget` in `--json`).

Every installed command is guarded: with clalt moved or removed, a hook is a
silent no-op, and the status line runs your own command directly (a copy of it is
in the command in `settings.json` for that case). The path is the stable link on
`PATH` when the running clalt sits in a versioned or temporary folder (a Homebrew
Cellar, a go build cache); install says which it used. `clalt feed status` and
doctor warn when the recorded clalt no longer exists.

Uninstall puts your file back byte for byte from the backup when it is still
exactly what install wrote. If you changed it since, it removes only the entries
with the marker, restores your statusLine and rewrites the file with sorted keys
and two space indent (the one formatting change; your values are kept as they
are) after backing it up too.

The installed commands do nothing, quietly, unless the session is a `clalt run`
session (`CLALT_SESSION`) or plain claude in a home that is matched to an account.
A session that has both the per-session settings and the global ones records
once, through `CLALT_SESSION`. A plain claude session is kept as
`ext-<claude session id>` in `~/.config/clalt/feed/sessions/` under the matched
account, shows in the dashboard Sessions view (it cannot be stopped from there)
and, with its status line, fills the limits of the account. It counts as running
while it has not ended and reported in the last 6 hours, and its feed file is
removed 24 hours after the last report.
