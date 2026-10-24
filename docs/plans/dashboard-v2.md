# Dashboard v2 plan

PR 1 shipped as PR #6. The PR 2 and PR 3 sections below are superseded by
docs/plans/clalt-v2.md (2026-10-19): thin profiles instead of per-account
configs, a session wrapper, and account switching.

Approved 2026-10-15 on branch feat/dashboard-v2. Replaces the internal/tui
dashboard with a focus-panes dashboard built on Bubble Tea v2 and Lip Gloss v2,
and moves logins to claude itself so sessions stay alive without clalt holding
tokens.

## Why

- The old dashboard was reskinned three times, but its key handling, data
  loading and action flow never changed. Known bugs: esc quits, enter tears the
  screen down to run claude, a full transcript rescan every 2s with no
  in-flight guard, dropped errors, logout deletes the account, and a made-up 5h
  percentage (tokens / 2,000,000) without --live.
- An env-injected CLAUDE_CODE_OAUTH_TOKEN is used for the whole session and is
  never refreshed (claude docs), so runs die at the roughly 8h token expiry.
  The supervisor kills and resumes claude instead, and the keepalive and refresh
  paths share single-use refresh tokens, which logs accounts out.
- On macOS, CLAUDE_CONFIG_DIR keys claude's Keychain entry to that folder
  (claude docs, authentication page). Each profile can own its own login, and
  claude refreshes it mid-session. Grants are capped at 30 days from sign-in
  regardless.

## Scope (confirmed)

- Focus-panes dashboard for one local user: pick and run the right account, and
  see limits, sessions, spend and account health. Opened by `clalt` and
  `clalt dashboard`.
- Accounts list: label, 5h usage, `!` for a limit near, a login due within 3
  days or a problem, running-session count, signed-out state, and a "most room
  left" line that ranks by the worse of the 5h and 7d windows. A list longer
  than the pane scrolls with the selection and ends in "+N more".
- Right pane views, switched with `[` `]` or 1-5:
  - Overview: identity, login health (re-login due), last used, 5h and 7d bars
    with reset clock time and countdown, pace forecast, data age, budgets,
    today and week usage, running summary.
  - Sessions: live state (working, waiting, idle), folder and branch, model,
    effort, context fill, duration, lines changed, estimated cost, stop.
  - Limits: 7-day chart of window readings, 7d pace, 5h run-out time.
  - History: Days (input, output, cache read, cache write, cost, sessions),
    Projects (with drill-in), Models.
  - Profile: folder, claude version, shared or isolated settings, login health,
    usage feed health, last used.
- Remove means sign out and keep history (`claude auth logout`, account hidden,
  folder kept).
- Rename changes the label only. The profile folder never moves.

## Delivery

PR 1, dashboard on demo data:
- internal/dashboard and internal/dashboard/demo, reachable via
  `clalt dashboard --demo`. The old dashboard stays the default.
- Demo data must be internally consistent: today, week, per-day, per-project,
  per-model and per-session totals all come from one demo ledger.
- Fixes found in the preview review: the Models column says "cost share", the
  Profile row is labelled "Usage feed", and signed-out accounts show a sign-in
  prompt in every view.

PR 2, claude-owned logins:
- `clalt login` creates the profile and runs `claude auth login` with
  CLAUDE_CONFIG_DIR set to it. Every existing account signs in fresh.
- `clalt run` stops injecting tokens and strips CLAUDE_CODE_OAUTH_TOKEN,
  CLAUDE_CODE_OAUTH_REFRESH_TOKEN, ANTHROPIC_API_KEY and ANTHROPIC_AUTH_TOKEN
  from the child env, saying which it stripped. Stop setting
  CLAUDE_SECURESTORAGE_CONFIG_DIR (undocumented, and same value as
  CLAUDE_CONFIG_DIR, so the Keychain key does not change).
- Remove the supervisor, keepalive, refresh command and the OAuth client. Leave
  the vault and keepalive files on disk, unused. Uninstall the keepalive job if
  present.
- The old dashboard keeps working on local transcript data, with its token
  expiry and keepalive panels removed.

PR 3, real data:
- internal/feed (ported from feat/clalt-office): `clalt statusline` records limit
  readings, `clalt hook` records live session state; installed per profile with
  chaining.
- Usage ledger with an incremental transcript cache (see the closed PR #4 branch
  fix/profile-full-config-sync), scanning ~/.claude/projects and every
  profile's projects folder.
- Real Source and Actions, make v2 the default, delete internal/tui.

## Design reference

docs/plans/dashboard-v2-frames.txt holds the approved frames in ascii mode.
- Layout: header line, Accounts pane (26 wide at 96+ columns, 22 at 72 to 95),
  account pane titled with the account label, footer key hints. Under 72
  columns one pane shows at a time. Under 60x16 a "terminal too small"
  message.
- Keys: j/k and arrows move, tab switches pane, `[` `]` and 1-5 switch view,
  h/l switch History tab, enter run (or open the project under the cursor in
  History > Projects), s sign in, e budget, r rename and d sign out (with the
  accounts pane focused), x stop, / filter, a ascii, ? help. Esc closes
  overlays and goes back from a project, and never quits. Backspace also goes
  back from a project. q quits unless typing. ctrl+c always quits.
- Color: ANSI 16 only. Bars green under 70, yellow 70 to 89, red 90 and up.
  Working green, waiting yellow, idle dim. Focused border default foreground,
  unfocused dim. Never color alone: `!`, the session dot, "signed out" and `~`
  for estimates carry the meaning too. NO_COLOR and CLALT_ASCII respected.
- Glyphs come from Go escapes and lipgloss borders, never literal characters
  in source files.

## DECIDE coverage

Applied: 2, 3, 4, 5, 7, 8, 10.
Skipped:
- 6: no external consumers. Removed commands get a CHANGELOG note in PR 2.
- 9: no flag system. PR 1 ships behind `--demo`. The old dashboard cannot
  survive PR 2's token removal, so rollback is reinstalling the previous
  build.
- Endpoint design: no HTTP API.

## Decisions

### Placement (stage 4)
Suggestion: new package internal/dashboard (model, views, widgets) and
internal/dashboard/demo. Account stays in internal/account. Totals live in
internal/usage behind one ledger. Feed parsing lives in internal/feed (PR 3).
internal/tui is deleted in PR 3.
Driven by: v1 and v2 Charm libraries must coexist until PR 3; new UI area with
its own use cases; the old dashboard stays the default meanwhile.
Rejected: rebuilding inside internal/tui. The v1 and v2 types collide, and the
old dashboard would break halfway through.
Vocabulary: Service is the clalt binary, App is a package under internal/,
Aggregate is the Account (folder, login state, label, budgets).

### Dashboard contract (stage 2)
Suggestion: internal/dashboard owns its view types and two ports.
`Source.Load(ctx) (Snapshot, error)` runs off the UI loop in its own goroutine,
one load at a time, and the dashboard stops waiting after 30s ("load timed out
after 30s") even when a source ignores ctx. When Load returns both a snapshot
and an error the error wins and the last good snapshot stays. `Actions` takes
ctx on every method. Run and SignIn return an `*exec.Cmd` that the dashboard
runs through tea.ExecProcess with the screen released, then reloads; (nil, nil)
and a command that cannot start are reported, not ignored. Rename, budget, sign
out and stop session return errors shown inline, and a `Notice` is a message
and not a failure. Adapters must be safe for concurrent use, since actions run
while a load is in flight, and a panic in one becomes an error message.
Accounts are addressed by stable id, and a load with an empty or a duplicate id
fails. Every total comes from one ledger.
The Snapshot holds absolute times (LastUsedAt, Login.SignedInAt and DueAt,
Limits.ReadAt, Window.ResetsAt, Session.StartedAt) and forecasts as data
(Window.Projected, Window.RunsOutAt), never prose or countdowns. The dashboard
words them against its own clock, in that clock's zone, so a snapshot that is
a minute old reads a minute older. Account.Problem is shown in the overview and
flagged with a red `!` in the list. Snapshot.Warnings show as one line under
the header. The dashboard cleans every text field of a snapshot (control
characters become spaces, escape sequences are removed) before it draws.
`Options.Logger` gets the full text of load and action failures, never
transcript text.
Driven by: demo and real data must be swappable, actions must not tear the
screen down, views must agree, and a source or adapter that misbehaves must
show up as a message instead of a frozen or corrupted screen.
Rejected: calling cli.App directly like today's Controller. That couples the UI
to cli and rules out demo mode.

### Data and migration (stage 7)
Suggestion: state.json gains id, label, hidden and signedInAt per account. For
existing accounts id is the current name, so profiles/<name> keeps working.
New accounts get id from the first label, with a numeric suffix on collision.
PR 3 adds feed/<id>/readings.jsonl (appended on change, trimmed to 30 days) and
feed/<id>/sessions/<session>.json. PR 2 only adds: the vault and keepalive files
stay on disk unused and get deleted in a later cleanup PR once PR 2 is proven.
Driven by: a profile folder must never move (claude's login and chat history
are tied to its path), and rollback must be safe because an older clalt
binary ignores the new fields.
Rejected: renaming the folder on rename. That loses the claude login and
history. Today's Store.Rename already orphans profile folders this way.

### Calling claude (stage 4, resilience)
`claude auth status --json`: 5s timeout per profile, at most 4 at once, cached
60s, refreshed right after sign in or out. No retry (the next refresh retries).
If claude is missing or older than 2.1.268: one banner, calls skipped. On
failure, show the last known status with its age. Sign in and out are
interactive and run through tea.ExecProcess.

### Feed contract (stage 5, local files)
claude owns the statusline and hook payload format. Parse leniently (ignore
unknown fields, tolerate missing ones) and bounds-check values (0 to 100
percent, sane timestamps). Readings are deduped on window, resets_at and
percent. Session state is keyed by session_id, and the newest timestamp wins.
Malformed input is dropped and counted in the log.

### Threat model (stage 3)
Applies because: authentication (profile logins), process control, local files
written by claude hooks.
1. A token in the environment overrides the profile login, so work runs on the
   wrong account. Mitigate (user choice): strip it and say so, PR 2.
2. Stop session signals a reused PID. Mitigate: check the process is claude and
   its start time matches the record before signaling, PR 3.
3. Installing the statusline clobbers the user's own. Mitigate: write only the
   profile's own settings file, chain the previous command, restore on
   uninstall, PR 3.
4. A same-user process writes fake feed files. Accept: same-user access already
   owns everything, and bounded parsing means bad data cannot crash the UI.
5. Prompts or paths leak into logs. Mitigate: log counts, durations and error
   kinds only, never transcript text, PR 3.
6. Huge or malformed transcripts freeze the UI. Mitigate: incremental cache
   keyed by path, size and mtime, a line length cap, loading off the UI loop,
   PR 3.
Residual risk accepted: 4.

### Accessibility (stage 8)
Keyboard only, help lists every key, esc never quits, ctrl+c always quits.
Focus shows as a brighter pane border plus the marker. ANSI 16 colors, never
color alone, NO_COLOR and ascii mode. Screen readers are out of scope for a
TUI. Verify with ascii golden frames, a NO_COLOR test, scripted key
walkthroughs, and a manual pass in kitty light and dark themes.

### Observability (stage 10)
- A source fails: error line in the dashboard plus ~/.config/clalt/logs/dashboard.log.
- Limits older than 1h: the age turns yellow.
- Feed not installed: shown in the Profile view and by `clalt doctor`.
- Slow scan: duration logged, spinner shown, the UI never blocks.
- Login near the 30-day cap (3 days left): yellow warning in the list and Overview.
`clalt doctor` gains checks for claude version, feed health and login status.
No paging: the dashboard is the alert surface for a single-user tool.

## User decisions

- Layout: focus panes, with every detail offered in all four areas.
- Build order: prototype first (PR 1 demo, PR 2 logins, PR 3 real data).
- Remove: sign out, keep history.
- Env token on run: strip it and say so.
- Migration: fresh sign-in for every account.
- Worktree: ~/Projects/personal/clalt-wt/dashboard-v2.

## Open assumptions

- internal/feed lands from feat/clalt-office before PR 3, or PR 3 ports it.
- fix/usage-accuracy (session-based attribution, pricing table, dedup) lands
  before the PR 3 ledger.
- The claude statusline keeps reporting rate_limits for the user's plans.
  Confirmed on 2.1.296 for a team plan.
- Session costs come from transcripts. The statusline reports $0 cost on
  subscription plans.

## Watch items for PR 3

From the PR 1 security review. None of these is exploitable on demo data.
- dashboard.Render (the non-TTY path) takes the snapshot without
  cleanSnapshot. Real data must go through cleanSnapshot there too.
- Labels may contain "/" and "..". Never build a path or a command argument
  from a label.
- Account and session ids are not cleaned, because they are never drawn.
  Adapters must validate them before using them in a profile path or a
  command argument.
- bubbletea v2.1.0 can leave its renderer stopped when an exec returns at
  once. The run adapter must resolve claude with exec.LookPath before
  returning a command.
