# clalt v2 step 3a: session registry and feed

Implements the first half of step 3 of docs/plans/clalt-v2.md. Steps 3b (dashboard on
real data), 4 (move, auto-switch) and 5 (alerts, resume, jump) consume it
through small interfaces of their own, so the names below are the contract.

## Scope

1. A session registry: `clalt run` gives each session a handle and keeps one record
   per running session, findable by handle.
2. A feed under ~/.config/clalt/feed/: per-session state and per-account limits.
3. `clalt hook <event>` and `clalt statusline`, which claude runs and which write the
   feed. They are installed per session with `claude --settings`; the user's
   ~/.claude/settings.json is never modified.
4. A doctor feed check.
5. If time allows: usage attribution by claude session id.

Not in scope: reading ~/.claude/sessions/<pid>.json (undocumented), alerts, the
dashboard adapters, move and auto-switch.

## Verified in the official docs (2026-10-23, claude 2.1.30x)

Sources: code.claude.com/docs/en/{hooks,statusline,cli-reference,settings,
settings-reference,env-vars,permissions}.

- `--settings` (cli-reference): "Path to a settings JSON file or an inline JSON string.
  Values you set here override the same keys in your settings.json files for this
  session. Keys you omit keep their file-based values. The file must be a regular file
  no larger than 2 MiB." Precedence (settings): managed, command line, project local,
  project, user.
- Hooks (hooks, settings-reference): "Hook entries merge across settings levels rather
  than replacing each other". The same handler defined in two files runs once. Matching
  hooks of an event run in parallel. `--settings` is a settings source, so the user's
  own hooks keep running. Not run-tested here (the real claude is off limits to this
  build); a manual check is in "Before release".
- statusLine is one object, so a `--settings` statusLine replaces the user's. That is
  why `clalt statusline` runs the user's own command itself. Optional keys: padding,
  refreshInterval, hideVimModeIndicator.
- A custom statusLine hides most footer hints (esc to interrupt, ? for shortcuts).
- Hook stdin fields. Common: session_id, transcript_path, cwd, hook_event_name.
  SessionStart: source (startup, resume, clear, compact, fork), model (may be absent).
  UserPromptSubmit: prompt. Stop: stop_hook_active, last_assistant_message. StopFailure:
  error (rate_limit, overloaded, authentication_failed, billing_error, ...).
  Notification: message, title, notification_type (permission_prompt, idle_prompt,
  elicitation_dialog, elicitation_url_dialog, agent_needs_input, ...). SessionEnd:
  reason (clear, resume, logout, prompt_input_exit, other).
- Stop does not run after a user interrupt. StopFailure runs instead of Stop on an API
  error. permission_prompt arrives about 6 seconds after the prompt shows, and only
  while the user has not typed. idle_prompt arrives 60 seconds after Stop.
- Hook stdout is added to claude's context for SessionStart and UserPromptSubmit, so the
  hook never prints. Exit 0 with no output is success. Hooks inherit claude's
  environment (so CLALT_SESSION), except that CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=1 also
  removes CLAUDE_CONFIG_DIR: hooks find the account through the registry, not the env.
- Timeouts: default 600 s (30 s on UserPromptSubmit). SessionEnd hooks share a 1.5 s
  budget, raised to the highest per-hook timeout in settings. async: true hooks run in
  the background and cannot block.
- statusLine stdin: model.id, model.display_name, cwd, workspace.current_dir,
  session_id, context_window.used_percentage (null early), rate_limits.five_hour and
  .seven_day with used_percentage (0 to 100) and resets_at (unix seconds). rate_limits
  exists only for claude.ai Pro and Max, only after the first API response, each window
  may be absent, and a window is dropped once its resets_at passes. The command runs at
  start and after each assistant message (debounced 300 ms); output counts only when
  the command exits 0; an in-flight run is cancelled by the next update.
- In an interactive session hooks and the statusLine wait for the workspace trust
  dialog, user settings included. Until then the session stays "starting".
- Shell form hooks run through sh -c. Exec form (`args`) exists, but a claude that does
  not know it would run the bare binary and print usage into the context, so shell form
  with safe quoting is used.

## Contracts

All JSON is camelCase and written atomically with mode 0600. Fields are only ever
added, never renamed.

### Registry: ~/.config/clalt/sessions/<handle>.json (package internal/registry)

```go
type Record struct {
    Handle          string    `json:"handle"`          // 16 hex chars, random
    Account         string    `json:"account"`         // account id
    Folder          string    `json:"folder"`          // its CLAUDE_CONFIG_DIR
    WrapperPID      int       `json:"wrapperPid"`      // the clalt run process
    PID             int       `json:"pid"`             // claude, 0 until started
    ClaudeSessionID string    `json:"claudeSessionId,omitempty"` // follows /clear and /resume
    Cwd             string    `json:"cwd"`             // where run was started
    Terminal        Terminal  `json:"terminal"`
    StartedAt       time.Time `json:"startedAt"`
}
type Terminal struct {
    Kind          string `json:"kind"`                    // kitty, tmux, ide, terminal
    KittyWindowID string `json:"kittyWindowId,omitempty"` // KITTY_WINDOW_ID
    KittyListenOn string `json:"kittyListenOn,omitempty"` // KITTY_LISTEN_ON
    TmuxSocket    string `json:"tmuxSocket,omitempty"`    // first field of $TMUX
    TmuxPane      string `json:"tmuxPane,omitempty"`      // TMUX_PANE
    IDE           string `json:"ide,omitempty"`           // vscode, cursor, jetbrains, zed
}
```

- Account, Cwd, StartedAt and PID keep the names of the v0.2 record, so
  `clalt sessions --json` and the old dashboard read the same file. profile.Session
  becomes an alias of Record and its functions delegate, so no old caller changes.
- Kind precedence: tmux (TMUX and TMUX_PANE) over kitty (KITTY_WINDOW_ID) over ide
  (TERM_PROGRAM=vscode, CURSOR_TRACE_ID, TERMINAL_EMULATOR=JetBrains-JediTerm, ZED_TERM)
  over terminal. Facts that are present are recorded whatever the kind; a kitty window
  id inside tmux may be stale, so consumers pick fields by Kind.
- A record is live while WrapperPID is alive (PID when WrapperPID is 0, a v0.2 record).
  Listing removes the files of dead records; doctor only reads (Scan).
- Writers take one lock file (sessions/.lock): the wrapper (create, set pid, remove)
  and the hooks (set claudeSessionId).

### Feed: ~/.config/clalt/feed/ (package internal/feed)

```go
type State string // starting, busy, idle, needs-you, ended

type SessionState struct { // feed/sessions/<handle>.json
    Handle     string    `json:"handle"`
    Account    string    `json:"account"`
    State      State     `json:"state"`
    Since      time.Time `json:"since"`               // when State last changed
    Reason     string    `json:"reason,omitempty"`    // needs-you: the notification type
    LastError  string    `json:"lastError,omitempty"` // StopFailure error, e.g. rate_limit
    Model      string    `json:"model,omitempty"`
    Cwd        string    `json:"cwd,omitempty"`       // follows claude's cwd
    ContextPct *float64  `json:"contextPct,omitempty"` // 0 to 100, from the statusline
    UpdatedAt  time.Time `json:"updatedAt"`           // last report of any kind
}
type Window struct {
    UsedPct  float64   `json:"usedPct"`  // 0 to 100
    ResetsAt time.Time `json:"resetsAt"`
}
type AccountLimits struct { // feed/accounts/<account id>.json
    Account    string    `json:"account"`
    FiveHour   *Window   `json:"fiveHour,omitempty"`
    SevenDay   *Window   `json:"sevenDay,omitempty"`
    ObservedAt time.Time `json:"observedAt"`
}
```

Beyond the brief: `since`, `lastError`, `contextPct`, `handle` and `account` in the
session file. Readers return missing or corrupt files as "not there", never as errors,
and a window whose resetsAt is past is expired (`Window.Expired(now)`).

### State machine (feed.ApplyHook)

| event | effect |
| --- | --- |
| SessionStart | idle (not for source compact, which fires mid reply); set model and cwd; record claudeSessionId in the registry |
| UserPromptSubmit | busy; clears reason and lastError |
| Stop | idle; clears reason and lastError. This is the pause point for step 4 |
| StopFailure | idle; lastError = error. A rate_limit here is "claude reports the limit" |
| Notification | needs-you with reason = notification_type, for permission_prompt, elicitation_dialog, elicitation_url_dialog, agent_needs_input; other types change nothing |
| PostToolUse, PostToolUseFailure | needs-you becomes busy; otherwise no write at all |
| SessionEnd | reason clear or resume: idle (the conversation goes on under a new id); else ended |
| `clalt run` start / exit | starting / ended, written by the wrapper itself |

Events on an ended session are ignored. Known gaps, stated for steps 4 and 5:

- A user interrupt (Esc) fires no event, so busy stays until the next prompt or Stop.
  Step 4 must not wait for Stop forever.
- needs-you is cleared by the next tool result, not by the user's answer. A permission
  notification within 6 s of a tool result is dropped as late.
- idle_prompt is not subscribed: flipping idle to needs-you after 60 s would hide the
  pause point from a move requested later.

### Rate-limit merge (feed.MergeLimits)

Several sessions of one account report, and an idle session re-sends old data. Per
window: ignore one whose resetsAt is past; a later resetsAt wins; on an equal resetsAt
the higher usedPct wins (usage only grows inside a window). ObservedAt is the latest
report. Written under the feed lock.

### Feed store behavior

- Lock: feed/.lock, 1.5 s wait in hooks (SessionEnd 0.7 s, statusline 0.3 s; a timeout is logged and
  the update skipped), 2 s elsewhere. Writes are temp file plus rename without fsync (9 ms per write with
  fsync on this Mac, 0.14 ms without): the feed is disposable. Added as
  fsutil.WriteFileAtomicNoSync.
- Prune(live): removes session files whose handle is not live, except `ended` ones
  younger than 10 minutes. Run when `clalt run` starts and by the SessionStart hook.
  Liveness is injected (the cli asks the registry), so feed does not import registry.

## Commands

`clalt hook <event>` (hidden from help): reads stdin (1 MiB cap, 1 s read deadline),
never prints, always exits 0, does nothing without a valid CLALT_SESSION, logs
failures to ~/.config/clalt/logs/hook.log (RFC3339 UTC first, rotated at 256 KiB).
The event comes from argv; the payload's own name is ignored.

`clalt statusline` (hidden): reads stdin, and with CLALT_SESSION updates the session
(model, context, cwd) and the account's limits. Then runs the user's own statusLine
command from the shared settings.json (3 s timeout, same stdin, inherited env) and
prints its stdout unchanged. Its exit code is passed on, so a failing user script looks
the same as without clalt. CLALT_STATUSLINE_CHAINED=1 is set for the child and stops a
loop. Without CLALT_SESSION it only passes through. With no user statusLine it prints
nothing.

## clalt run changes (internal/cli/cmd_run_feed.go, small hooks in cmd_run.go)

1. handle, terminal, record written before claude starts (a failure warns, the
   session still starts), feed state `starting`.
2. CLALT_SESSION=<handle> added to claude's env (any inherited value replaced).
3. `--settings <json>` put first in claude's args. Skipped, with one stderr line, when
   the user passes their own --settings or the clalt binary path is unknown.
4. After start the record gets claude's pid. On exit: feed `ended`, record removed.

Installed events and timeouts (seconds): SessionStart 5, UserPromptSubmit 5, Stop 5,
StopFailure 5, Notification 5 (matcher: the four needs-you types), SessionEnd 1 (stays
inside the 1.5 s budget so the user's own exit hooks keep their timing), PostToolUse and
PostToolUseFailure async. Command: `'<abs clalt>' hook <Event>`, POSIX single quote
quoting (`'` becomes `'\''`). The statusLine command is `'<abs clalt>' statusline` and
copies padding, refreshInterval and hideVimModeIndicator from the user's statusLine so
the look stays. The JSON is built with encoding/json, sorted keys, no HTML escaping.

## Decisions and why

- New package internal/registry, with a thin shim in profile: nothing else changes in
  files that other streams (old dashboard wiring) may touch.
- Keyed by handle, not pid: the record exists before claude does, and a move (step 4)
  keeps the handle while pid and account change.
- The statusline runs only the user's own ~/.claude/settings.json statusLine, never a
  project file: a project's command waits for workspace trust and clalt would run it
  without. A project that defines its own statusLine loses it under clalt run (ours
  outranks it). Hooks of projects keep running.
- One lock per directory instead of one per record: no lock files to clean up, the
  critical sections are microseconds.
- The wrapper writes `ended` itself, since claude may die without SessionEnd.
- Extra events beyond the brief (StopFailure, PostToolUse, PostToolUseFailure) exist
  because without them rate-limit stops and answered permission prompts leave the
  state wrong. Each is one row of the table in cmd_hook_events.go.

## PR plan

One PR, commits in this order, each test first: plan doc; fsutil NoSync; registry;
run uses the registry and sessions --json grows; feed store; hook; statusline;
--settings install; doctor and docs; usage attribution if there is time.

## Before release (manual, with a real claude)

Run `clalt run work`, and confirm in a scratch project: the user's own hook and
statusline still work; `clalt sessions --json` shows a claudeSessionId after the first
prompt; feed/sessions shows busy then idle; an Esc interrupt leaves busy; a permission
prompt turns needs-you after about 6 s and busy after the tool runs.

## Open questions for the user

- With no statusLine of your own, ours hides the footer hints and shows a blank row.
  Print a short "account 5h N% 7d N%" line instead?
- A user-supplied --settings turns the feed off for that session. Merge instead?
- Keep the three extra events?
- Should `clalt sessions` also show the feed state?
- Stale pids after a reboot can keep a dead record alive until its pid is free again;
  a boot-time check would fix it.
