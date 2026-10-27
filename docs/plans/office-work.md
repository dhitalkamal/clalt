# office work: activity, actions, warnings, reviews, summary, history

the office becomes a place to work from, not only to watch. six features,
agreed with the user on 2026-10-27, shipped as four pull requests.

## decisions (agreed with the user)

- activity log shows everything: prompts, replies, tool calls and results. it
  is read from Claude Code's own transcript files when the user opens a log;
  clalt never copies or stores that text. the daily summary is built the same
  way, on demand.
- session actions from the office: move to another account (existing clalt
  move), open its terminal (existing terminal focus), stop the session (asks it
  to end cleanly, always behind a confirm step in the page). all three are
  write routes behind the existing guards (token, Host, X-Clalt-Edit header,
  same-origin, content type).
- limit warnings: forecast when an account hits its 5h limit at the current
  pace, shown on the account card, a desktop notification through the existing
  notify package, and a one-click move to the account with the most room.
- pr reviews: clalt watches pull requests where the user's review is
  requested, reviews each one with a headless Claude run in a read-only
  sandbox, never posts anything to GitHub, and keeps the findings plus the full
  run log; the office shows them at the reviewer's desk. no daily cap (huge PRs
  over about 1500 changed lines are skipped). GitHub access only through the
  `gh` CLI, clalt never sees a token. repos are opted in from Settings.
- daily summary and usage history charts live in the office.

## delivery

1. activity log (this branch, feat/activity-log): the base the others use.
2. then in parallel, each its own branch off develop after 1 merges:
   a. session actions and limit warnings,
   b. pr reviews,
   c. daily summary and usage history charts.
the contracts for 2a to 2c are written here before their streams start.

## contract 1: activity log

GET /api/activity?handle=<session handle>[&agent=<subagent id>][&before=<cursor>][&limit=<n>]

- read route: same guards as GET /api/snapshot (token cookie, Host check).
- handle is a session handle from the snapshot; agent narrows to one
  subagent's run (its sidechain); without agent the main conversation is
  returned with subagent runs as start and end markers.
- limit default 100, max 300; entries come newest first; `next` is an opaque
  cursor for older entries, "" when there are none.
- reply:
```
{"handle": "...", "agent": "" or id, "next": "cursor or empty",
 "entries": [
   {"t": "2026-10-27T14:03:11Z",
    "kind": "prompt|reply|tool|result|subagent-start|subagent-end|note",
    "text": "full text, capped at 20000 chars with a cut marker",
    "tool": "Read|Edit|Bash|Agent|... or empty",
    "files": ["relative/or/absolute/path", "..."],
    "durationMs": 0,
    "ok": true,
    "agent": "subagent id for sidechain entries or empty"}
 ]}
```
- source: the session's transcript JSONL under the account's Claude config
  folder (the same rules internal/usage and internal/conversation use to find
  transcripts); subagent sidechains as Claude Code writes them (in the same
  file with isSidechain, or in a per-session subagents folder: support what
  the current Claude Code writes, check real files read-only to learn the
  shape, never copy real content into tests).
- strings cleaned of control and format characters (keep newlines and tabs),
  every field size-capped, a whole reply capped at 2 MB; unreadable or
  malformed lines are skipped and counted in a `skipped` field.
- reading is bounded: at most 64 MB scanned per request, reading backwards
  from the end so newest entries are cheap.

page: clicking a session character, a team desk or an intern opens an
activity panel (side drawer) with the timeline: prompts and replies as text
blocks, tool calls as compact rows (tool, files, duration, ok), subagent runs
as collapsible groups, "load older" paging, auto-refresh while open (every 3 s,
only fetching entries newer than the newest shown), Esc closes, keyboard
reachable, works at 420 px.
## contract 2a: session actions and limit warnings

routes (write routes: token, Host, X-Clalt-Edit header, same-origin, JSON content type, body cap):
- POST /api/session/move {"handle": "...", "account": "<account id>"} -> 202 {"queued": true} (uses the clalt move request; the wrapper moves at the next pause), 409 {"reason"} when not possible.
- POST /api/session/focus {"handle": "..."} -> 200 {"focused": true} | 409 {"reason"} (terminal.Driver.Focus).
- POST /api/session/stop {"handle": "..."} -> 202 {"stopping": true} | 409 {"reason"} (asks the clalt run wrapper to end the session cleanly, like Ctrl-C once; never kills other processes).
- the snapshot gains per session `"actions": {"move": bool, "focus": bool, "stop": bool, "why": "short reason when false"}`; sessions started with plain claude (ext- handles) get all false with why "started outside clalt: use clalt run to control it".
- the page: clicking a session opens the activity drawer; its header gets Move to (account picker, accounts with their 5h percent, best one first), Open terminal, Stop (confirm step: "Stop <project> on <account>? It ends after the current step."). Disabled buttons show the why.

limit warnings:
- the snapshot gains per account `"forecast": {"fiveHourAt": "RFC3339 or null", "pacePctPerHour": n}` from the recent usage pace (reuse the dashboard pace code), null when not enough data or no live session.
- the account card shows "full in ~25m" when fiveHourAt is within 60 minutes, amber under 60, red under 20.
- a desktop notification through internal/notify when an account crosses the 20-minute mark (once per window), text "WORK hits its 5h limit in about 20 min. Move sessions to PERSONAL (12%)." clicking the card's "Move all" moves every movable session of that account to the account with the most room (calls the move route per session, confirm step first).

## contract 2b: pr reviews

- Settings gains "PR reviews": on/off, repos list (owner/name, opted in one by one, filled from `gh repo list` suggestions), shows when it last checked. stored in ~/.config/clalt/reviews/config.json (0600).
- a watcher in the office server process (only while clalt office runs; later maybe in clalt feed) polls every 5 min with `gh search prs --review-requested=@me --state=open --json ...` limited to the opted-in repos; clalt never reads or stores a GitHub token, only runs gh.
- each new PR (or new head commit) gets a review run: checkout with `gh pr checkout` into a temporary worktree of a clone under ~/.cache/clalt/reviews/<owner>/<repo> (created once with gh repo clone), then a headless claude run on the account with the most 5h room: `claude -p` with the code-reviewer agent prompt, tools limited to Read, Grep, Glob (no Bash, no Write, no network tools), the diff and changed files as input; skip PRs over 1500 changed lines with a note. one review at a time, no daily cap.
- results stored in ~/.config/clalt/reviews/<owner>-<repo>-<number>-<sha7>.json (0600): {repo, number, title, author, url, sha, startedAt, endedAt, account, findings: [{severity, file, line, title, detail}], summary, transcript: the claude session id so the activity reader can show the full run log}.
- never posts anything to GitHub (no gh pr review, no gh pr comment, no API writes: enforced by a test that the code never builds those argv).
- GET /api/reviews -> list, GET /api/reviews/<id> -> one; the snapshot marks the reviewer desk busy while a review runs; the page shows a "Reviews" inbox on the reviewer desk (count badge), each review with findings and an "Open run log" button using the activity panel.

## contract 2c: daily summary and usage history

- GET /api/summary?day=YYYY-MM-DD (default today, local time): per account and per project: sessions, subagent runs by type, tool counts, files touched (top 20), time active, tokens and cost from internal/usage, notable failures; built on demand from transcripts and the usage aggregation, nothing stored.
- GET /api/history?days=30: per day per account tokens, cost, 5h and 7d peak percent (reuse internal/usage and the dashboard history code).
- the page: a "Today" button in the controls row opens a summary panel (day picker, per account and project sections, copy as text button); a "History" tab in the same panel shows charts per account (tokens per day, 5h peak per day) drawn with canvas or SVG, no external libraries.
