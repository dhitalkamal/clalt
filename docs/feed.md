# The session feed

`clalt run` keeps a record of each session it starts, and claude tells clalt what
the session is doing. Nothing here changes your own claude settings: the hooks and
the status line are given to claude with `--settings` for that session only.

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
Sessions that were not started by `clalt run` stay unattributed.
