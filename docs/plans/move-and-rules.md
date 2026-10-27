# clalt v2 step 4: move, account rules and auto-switch

Drafted 2026-10-23. Implements step 4 of docs/plans/clalt-v2.md. It is built next
to step 3a (session registry, feed, `clalt run` wrapper), which is not merged, so
everything here is written against small interfaces and tested with fakes. The
adapters to 3a's real types are a separate wiring pass.

## Scope

1. Account rules in `state.json`: the folders an account may serve, and whether
   auto-switch may use it. `clalt rules <name>` shows and edits them. `clalt
   doctor` reports rule folders that no longer exist.
2. `clalt run` without a name picks the allowed account with the most room.
3. `clalt move <session> <account>` asks the running wrapper to move the session at
   its next pause, to the other account, with `claude --resume`.
4. Auto-switch: the wrapper moves a session by itself when its account runs low,
   only between accounts that opted in and only across folders the rules allow.

Not in this PR: the dashboard, kitty or tmux handling, needs-you alerts, a user-set
threshold, a cancel command for a pending move, Windows moves.

## What I need from step 3a

The contract from the brief, as the Go interfaces the wrapper package defines. The
types are mine and minimal, and 3a's adapters map onto them.

```go
type Registry interface {
    List() ([]Record, error)               // running sessions
    Dir() string                           // where the records live
    Update(handle string, c Change) error  // account, folder, claude pid
}
type Feed interface {
    Session(handle string) (SessionInfo, bool)  // state, limit hit, cwd, updated-at
    Limits(accountID string) (pick.Limits, bool)
}
type Accounts interface {
    Candidates() ([]pick.Candidate, error)      // accounts with their rules
    Prepare(id, cwd string) (Prepared, error)   // rules, link sync, sign-in check
}
type Launcher interface{ Start(folder string, args []string) (Process, error) }
type Process interface{ PID() int; Wait() error; Terminate() error; Kill() error; Signal(os.Signal) error }
type Terminal interface{ Restore(forced bool) }
```

Record is handle, account id, folder, wrapper pid, claude pid, claude session id,
cwd, terminal, started-at. SessionInfo is state (starting, busy, idle, needs-you,
ended), whether claude reported the limit, cwd and updated-at. Things the feed has
to do for this to work, to settle in the wiring pass:

- idle must stay idle after the Stop hook until the next prompt, and an
  idle_prompt notification must not turn it into needs-you (or the adapter maps it
  back), because only idle is a pause.
- the adapter sets LimitHit when claude stopped on a rate limit (needs-you reason).
- the old claude's SessionEnd makes the handle `ended`, and the resumed one starts
  at `starting` under the same handle. The wrapper ignores any observation older
  than the start of the current claude, so the stale `ended` or `idle` is never read.
- a resumed session that has not answered anything yet is `starting`, so it is not
  a pause until its first reply. That is also a natural cooldown.

## Data formats

### state.json

One additive top-level key, `rules`, a map from account id to that account's rules
(left out while no account has any, so v0.1.0 still reads the file; the compat
test is extended to prove it):

```
"rules": {"work": {"folders": ["/Users/k/Projects/work"], "autoSwitch": true}}
```

It is a map of its own and not two fields of Meta because tests of many commands
compare a Meta with `==` ("the refused command changed nothing"), and a slice in
Meta would break each of them, and any that other branches add. Rules are also
policy the user sets, where Meta is what clalt recorded. An account whose rules
are cleared loses its entry. An empty or absent `folders` means the account may
serve any folder. Entries are absolute and cleaned. `clalt rules` makes a
relative argument absolute against the working directory. Entries that are not
absolute never match, and doctor says so.

### Folder match

A rule matches that folder and everything under it. Matching goes through
fsutil.Within: the file system decides, so symlinks and the case folding of macOS
and Windows file systems are seen through, and when a path does not exist it falls
back to comparing resolved paths. The working directory that is checked is the one
the session is in (the feed's cwd when known) and the one claude is started in
(the record's cwd); both must be allowed.

### Move request

A file next to the session record: `<registry dir>/<handle>.move`. The extension is
not .json on purpose, so a registry that lists `*.json` and prunes what it cannot
read never sees it. Mode 0600, written atomically (temp file and rename).

```
{"version":1,"to":"<account id>","requestedAt":"2026-10-23T12:00:00Z"}
```

`to` is the account id, not the label. The handle is checked to be 16 hex
characters before it becomes part of a path. The wrapper polls the file every
500 ms (works where unix sockets do not). It claims a request by renaming it to
`<handle>.move.claimed`, so a newer request written during a move is kept for the
next pause. The wrapper removes both names when it ends. A newer request replaces
an older one that was not claimed yet.

### Resume command line

`claude --resume <claude session id>`, plus the flags of the original launch that
the docs say a resume does not restore: --mcp-config, --strict-mcp-config,
--settings, --plugin-dir, --fallback-model, --add-dir, and the bypass flags
(--dangerously-skip-permissions, --allow-dangerously-skip-permissions,
--permission-mode bypassPermissions), since a resume comes back in the default mode
for bypass. Not carried, on purpose: --model and --permission-mode (a resume
restores what the user changed in the session, and the launch value would undo it),
--agent, the system prompt flags (recorded in the session), --name, the first
prompt, and any of --resume, --continue, --session-id, --fork-session.

## Decisions

### Rules

- Rules decide automatic choices: picking without a name, and moves. An explicit
  `clalt run <name>` is not blocked by them (open question 3).
- `clalt rules <name> [--add-folder P]... [--remove-folder P]... [--auto-switch |
  --no-auto-switch] [--json]`. Removing the last folder means any folder again, and
  the output says so. Turning auto-switch on prints the terms line (below).
- Matching lives in internal/rules, apart from the store, so the account package
  only keeps strings.

### Picking an account (`clalt run` with no name)

- Candidates: not hidden, a sign-in recorded in state, folder rules allow the cwd.
  The winner is then checked with claude (same live check `run` does), and the next
  one is tried if it is not signed in. The picker ignores the auto-switch flag: that
  flag is about moving a running session.
- Room is 100 minus the higher of the 5-hour and 7-day used percentage. A window
  whose resets-at has passed counts as 0 used. Accounts with data come first, most
  room first. Accounts without recent data (observed more than 24 hours ago, or
  never) rank after them, and ties go by label. Why 24 hours: used percentage only
  moves with usage, and usage through claude updates the feed, so old data is still
  a good lower bound. The cost is web chat use, which the feed cannot see.
- It says on stderr which account it picked and why, and when nothing is allowed it
  lists each account with the reason it was left out.
- The logic is internal/pick. cmd_run.go gets one call, and a nameless `run` or
  `run -- args` is no longer a usage error. A flag before the name still is.

### Ending claude at a pause (research)

Sources: code.claude.com/docs/en/cli-reference, /sessions and /headless, read
2026-10-23. claude 2.1.296 is what the plan verified on this Mac. I never ran
claude, as instructed.

- `--resume <id>` finds the session from any directory (current project and its
  worktrees first, then every project). The conversation, model, agent and the
  permission mode come back, except bypassPermissions, and the session id is kept
  unless --fork-session is given. Flags such as --mcp-config, --settings,
  --plugin-dir, --fallback-model and --add-dir are not restored. A tool that was
  running when the process ended is shown to claude as cut off.
- The only documented way to end claude from outside is a signal. The docs describe
  SIGTERM for `claude -p`: exit code 143, the process tree of running Bash commands
  is ended, SessionEnd hooks run, no new tool call or model request starts, and the
  interrupted turn is left as it is on resume. Ctrl-C and Ctrl-D are key presses in
  the terminal, and clalt does not own a pty, so it cannot type them.
- Decision: SIGTERM at a pause, 15 seconds of grace, then SIGKILL. SIGINT is the
  "interrupt or clear input" key and needs a second press to exit, SIGHUP means the
  terminal is gone. At a pause the transcript is already complete (every reply is
  saved as it is written), so even a kill loses no conversation.
- Unverified: the docs describe SIGTERM only for -p. That interactive claude
  restores the terminal on SIGTERM is not documented. So the wrapper saves the
  terminal settings (termios) before the first start, restores them after every
  claude, and after a kill it also writes the reset sequences for bracketed paste,
  focus and mouse reporting, the kitty keyboard mode, the cursor and the alternate
  screen. One manual check with the real claude is needed before the wiring pass
  turns this on by default (open question 5).
- What a move costs the user, and the docs say it: an unsent draft in the prompt
  box is lost, background commands and subagents of the session end, and the new
  account re-reads the whole context (no shared prompt cache), which uses part of
  its limit.

### Move

1. `clalt move` finds the session (handle, claude session id prefix of at least 4
   characters, or wrapper pid), finds the account by label, and refuses when the
   session is already there, the target is hidden or not signed in, the target's
   rules do not allow the cwd, the record has no claude session id, or the feed has
   no state for the session (the pause could not be seen). Then it writes the
   request and says it will move at the next pause.
2. At a pause (state idle, not changed for 1.5 seconds, and observed after the
   current claude started) the wrapper claims the request and prepares the target
   first, while claude still runs: rules, link sync, projects check, auth-variable
   strip, sign-in check. Output of that step is held, since the claude screen owns
   the terminal.
3. Only then it ends claude, restores the terminal, prints one line, prints what the
   sync held, and starts the resume under the target.
4. If the start fails, it resumes under the old account and says so. If that fails
   too it prints the command to resume by hand and exits 1.
5. A failure while preparing drops the request and prints one line when claude ends
   (the terminal is not free before). An auto-switch target that fails is skipped
   for the cooldown.
6. The record is updated (account, folder, claude pid). A signal to the wrapper
   during a move that means quit (SIGTERM, SIGHUP) stops the move and ends the
   wrapper, with the code a shell would report.

The notice line: `clalt: moved session 3fa9c1d2 from work to personal: <why>`.
Why is "requested with clalt move", "5-hour window at 93% (switches at 90%)" or
"claude reported the limit".

### Auto-switch

- Off unless `autoSwitch` is on for the account. The session's account and the
  target both need it on.
- Trigger: the 5-hour used percentage of the session's account reaches the threshold
  (90), or claude reported the limit. The 7-day window only matters through the limit
  report and through the room used to rank targets.
- Target: allowed by the rules for the cwd, not hidden, signed in per state,
  auto-switch on, with recent data, and room at least 20 points above the current
  account's (0 room when the limit was reported). Best room wins, ties by label.
  Nothing qualifying means no switch, silently.
- Guards: no auto-switch within 30 minutes of the last move of the session (manual
  moves count), and a target whose preparation failed is skipped for the same time.
  The threshold, margin and cooldown are constants in the wrapper config, not
  settings.
- It moves at the next pause, the same way as a requested move.

### Terms

Auto-switch is off by default. It is meant for accounts you are entitled to use for
the work at hand, for example your own personal and your own work account, and each
account's terms of service apply to how you use it. Moving a session between
accounts does not change that. docs/move.md says this, and turning auto-switch on
prints one line pointing at it.

### Threats

- A local process writes a request to move a session. It would need the same user
  (the folder is 0700, the file 0600), which could already kill claude or run `clalt
  move`. No boundary is crossed. The target is still checked against the rules and
  the sign-in when the request is executed, not only when it was written.
- The handle and ids end up in file names: the handle is validated as 16 hex, the
  target has to be an account in state.json, and nothing from a request is run.
- Rules are the policy. They are checked when the request is made and again at
  execution, and auto-switch never passes a rule.
- No credential is read or printed. Env var names are shown, never values, as in run.
- Loops: the cooldown, the margin and the fact that only idle sessions move.

## Defaults

Auto-switch off. Folder rules empty (any folder). Poll 500 ms. Settle 1.5 s. Grace
15 s. Threshold 90 percent of the 5-hour window. Margin 20 points. Cooldown 30
minutes. Recent data 24 hours.

## PR plan

One PR, feat/move-rules into develop, commits in this order: this plan; rules
package; account fields; `clalt rules` and the doctor check; picker; `clalt run`
without a name; wrapper (types, request, resume args, match, policy, loop, process
and terminal adapters); `clalt move`; docs and changelog. Every behavior test-first.

Built here and not wired: internal/wrapper with its fakes, ExecProcess (SIGTERM,
kill) and the terminal restore; `moveAccounts` in internal/cli, the Accounts adapter
(rules, link sync, projects check, strip note, sign-in check, notes held back).

Wiring pass (after 3a merges, its own PR): adapters from 3a's registry and feed to
the Registry and Feed interfaces, and for the seams `openRegistry`, `openFeed` and
`feedLimits` in internal/cli; a Launcher around claudecli.Session that sets
CLALT_SESSION; cmd_run.go starts the session through wrapper.Run instead of
runClaude, with its signal channel and the Notice printing to stderr (the relay
in wrapper.Run replaces relaySignals); cmdRun lines 46 to 69 could share a prepare
step with moveAccounts.Prepare.

## Open questions

1. Should the threshold, margin and cooldown be per-account settings, and a flag on
   `clalt rules`?
2. An exhausted account with data (room 0) ranks before an account with no data,
   by the literal rule. Rank it last instead?
3. Should `clalt run <name>` refuse or warn when the cwd is outside that account's
   folder rules? It does neither now.
4. The picker ignores the auto-switch flag. Right?
5. One manual check of SIGTERM against the real interactive claude (terminal
   restored, transcript whole, exit code), before the wiring pass enables moves.
6. The feed points above (idle stays idle, idle_prompt, LimitHit, resumed state).
7. 24 hours as "recent data" for ranking.

## Risks

- SIGTERM on interactive claude is unverified (above).
- A pause can be lost to a quick next prompt: the state is re-read just before the
  signal, and a prompt that slips through is left as an interrupted turn on resume.
- Moves need SIGTERM, which Windows has no way to send, so `clalt move` refuses
  there and the wrapper never moves.
