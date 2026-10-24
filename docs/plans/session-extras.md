# clalt v2 step 5: session extras

Drafted 2026-10-23 for step 5 of docs/plans/clalt-v2.md, from the step 5 brief. The
scope and the defaults were decided by the user. Everything under Decisions is the
builder's call, with the reason, and open to change in review. Step 3a (session
wrapper, registry, feed, hook handler) builds in parallel and is not merged, so what
needs it sits behind small interfaces of this PR's own.

## Scope

1. Resume anything: `clalt resume` lists every conversation in the shared projects
   folder, across accounts. `clalt resume <id or prefix> [--account <name>]` continues
   one under an account, from the folder it ran in.
2. Launch: `clalt launch <account> [--cwd dir] [-- claude args]` opens a new kitty tab
   or tmux window that runs `clalt run <account>`.
3. Jump: `clalt jump <session>` focuses the terminal a running session lives in.
4. Needs-you alerts: a desktop notification when a session waits for the user. This PR
   builds the notifier, the rate limit, the off setting and the function that step 3a's
   hook handler calls. It does not touch the hook handler.

Not in this PR: the adapters to step 3a (registry, feed, hook handler), step 4's account
picker, the dashboard actions. See Wiring pass.

## What step 3a has to provide

This PR defines these with its own minimal types and tests against fakes. It imports
none of 3a's packages.

- `terminal.Session`: the part of a registry record that jump and resume read: handle,
  account id, claude session id, cwd, terminal location (kind kitty, tmux, ide or
  terminal; kitty window id and listen-on address; tmux socket and pane; ide name),
  started-at.
- Running sessions, dead ones already pruned: `sessions() ([]terminal.Session, error)`.
  A stale record would focus the wrong kitty window, because window ids start again
  when kitty restarts.
- The account that last ran a conversation, when the feed or registry knows:
  `lastAccount(claudeSessionID string) (accountID string, ok bool)`. It returns an
  account id, not a label.
- In package notify, for the hook handler:
  - `Alerts.NeedsYou(ctx, Event{Handle, Account, Reason, Cwd}) (Outcome, error)`, called
    when a Notification hook puts a session into needs-you. It applies the off setting
    and the rate limit itself. Its error must never fail the hook.
  - `Alerts.EndWait(handle) error`, called on every other hook event of that session (a
    prompt was submitted, a tool ran, the turn stopped, the session ended). That is what
    ends a wait, so the next one may notify.
- Formats relied on: a handle is 1 to 64 letters, digits, dots, dashes or underscores
  (it becomes a file name). The kitty window id is KITTY_WINDOW_ID as a number, the
  kitty address is KITTY_LISTEN_ON as is (unix:... or tcp:...), the tmux pane is
  TMUX_PANE (%5), and the tmux socket is the path in TMUX, which is everything before
  the last two comma-separated fields.
- Where a session is, when both are set: tmux wins over kitty. Launch uses the same rule.

## Defaults (decided by the user)

- Needs-you notifications are on, at most one per session per wait, with a setting that
  turns them off.
- Launch and jump use only what is already there: kitty remote control on a socket
  (KITTY_LISTEN_ON is set) or a tmux server (TMUX is set). clalt never turns on kitty
  remote control and never changes kitty or tmux config. With neither, launch prints the
  command to run.

## Decisions

### Conversations

- D1. One package, internal/conversation, reads
  `<shared home>/projects/<project>/<uuid>.jsonl`. Only a regular file directly inside a
  project folder, named like a UUID and holding at least one user or assistant record,
  is a conversation. Nested folders (subagent data) are ignored. Why: that is what
  claude writes. 584 files on this Mac, all UUID named, 219 nested folders, 1 file
  without messages.
- D2. Title, first found wins: the name the user gave the session (record `custom-title`,
  key `customTitle`), claude's generated title (`ai-title`, `aiTitle`), a legacy
  `summary` record, then the first prompt. The last record of a kind wins. Why: the brief
  says summary record, else first prompt. On this Mac no file has a summary record.
  Names (22 files) and generated titles (50 files) are what claude writes now, and they
  are what claude's own picker shows first (docs: sessions), so the list reads like
  claude's.
- D3. First prompt: the first user record that is not meta, not a sidechain, not a compact
  summary, has no origin other than human, carries a string or a first text block, and
  whose text does not start with "<" (command and system wrappers). Whitespace is
  collapsed and the text is cut to 100 characters at a word boundary, ending in "..."
  when cut.
- D4. cwd: of the distinct cwd values in a file, the first whose encoding equals the
  project folder name, else the first. The encoding turns every character outside
  A-Z, a-z and 0-9 into a dash (docs: sessions, where the long-name rule is also
  stated: over 200 characters it is cut and gets a hash, and then only the first cwd is
  left). Why: claude looks a session up by that folder. On this Mac the first cwd
  encodes to the folder in 120 of 120 recent files, and 3 of 120 hold more than one cwd.
- D5. Model: the last assistant record that is not `<synthetic>`. Last activity: the
  newest timestamp of any record, else the file's mtime. Size: bytes on disk. JSON also
  carries claude's `entrypoint` (cli or sdk-cli).
- D6. A reader of its own, not internal/usage. `usage.ParseTranscript` drops every line
  without token usage, so it never sees titles, prompts or record types, and calling it
  too would read each file twice for fields the same pass already has. The reader keeps
  usage's conventions: malformed lines are skipped, lines up to 16 MB are read (a longer
  one is skipped, not fatal), the project label is the folder name. internal/usage is
  not changed.
- D7. Cost. Listing stats every file, orders by mtime (migrate keeps mtimes) and parses
  only as many files as it needs: newest first until `limit` match. The result is then
  sorted by last activity. A parse reads a whole file once (median 0.44 MB, p90 0.74 MB,
  max 18 MB here). The slow case is a filter that matches little, or `--limit 0`: every
  file once, about 400 MB here.
- D8. Text output: id (first 8 characters), last active (relative), model, size, folder
  (home shown as ~, cut from the left), title. Everything read from disk goes through
  clean() before it is printed. `--json` is an array of id, title, cwd, model,
  lastActive (RFC 3339 UTC), size, project, entrypoint. Default limit 20, `--limit n`
  changes it, 0 shows all. The filter is words, and every word has to appear
  (case-insensitive) in the id, title, folder or model.

### Resume

- D9. One argument of at least 8 characters made only of 0-9, a-f and dashes is a
  conversation id or prefix and runs it. Any other arguments are a filter and list.
  An id-shaped word that matches nothing is an error, not a filter, and an ambiguous
  prefix lists its matches and stops. Why: the brief gives both forms the same
  positional slot, the list prints 8 characters so what it shows is what runs, and
  running claude must never be the side effect of a mistyped filter.
- D10. Account: `--account <name>`, else `lastAccount` when it names an account that
  exists and was not removed, else stop with exit code 2, say to pass `--account` and
  list the accounts. No prompt: step 4's picker plugs in at that point during wiring.
- D11. Same path as `clalt run <name> -- --resume <id>`: resume changes into the
  conversation's cwd (and sets PWD to it), calls cmdRun with those arguments, and
  changes back. cmd_run.go is not edited (3a is rewriting it) and every check run makes
  still applies. A cwd that is missing or not a folder stops the command with a message
  and runs nothing. The message gives the way out: `clalt run <name> -- --resume <id>`
  from the folder wanted. Why the cwd: `claude --resume <id>` looks in the current
  project and its worktrees first and then in every other project (since 2.1.223, docs:
  cli-reference), so running from the original cwd makes the first, plain lookup hit
  and keeps the session in its own project folder.
- D12. Addition, easy to drop: when the registry shows a running session with the same
  claude session id, resume stops and names `clalt jump <handle>`. Two terminals on one
  session interleave into one transcript (docs: sessions). The way around it is
  `clalt run <name> -- --resume <id>`, which the message names.

### Launch

- D13. Where: tmux when TMUX is set and tmux is on PATH, else kitty when KITTY_LISTEN_ON
  is set and kitten is on PATH, else print the command. Why tmux first: it is the
  innermost one, so in tmux inside kitty the user's context is the tmux window.
  - tmux: `tmux new-window -P -c <dir> <command...>`. The command is given as separate
    arguments, which tmux 2.0 and later runs directly, without sh -c, so nothing is
    quoted. `-P` prints session:window of the new window.
  - kitty: `kitten @ --to <KITTY_LISTEN_ON> launch --type=tab --cwd=<dir> <command...>`.
    The documented spelling is `kitten @`, not `kitty @`. It prints the new window id.
- D14. The command is `env NAME=value ... <clalt path> run <account> [-- claude args]`.
  kitty and tmux start it with their own environment, not the caller's, so PATH (to find
  claude), XDG_CONFIG_HOME, CLAUDE_CONFIG_DIR and CLALT_CLAUDE_BIN are passed on through
  env when set, since another XDG_CONFIG_HOME would show another set of accounts. Only
  those four, never the whole environment. The clalt path is os.Executable. The printed
  fallback is the plain `clalt run <account> ...`, quoted for a POSIX shell where needed.
- D15. Before a tab opens: the account exists and was not removed, and it is signed in as
  recorded (the status check run makes), because a tab whose command fails closes with it
  and the error is lost. `--cwd` has to be an existing folder, default the current one.

### Jump

- D16. `<session>` is a handle (exact, else a unique prefix) or a prefix of a claude
  session id. Several matches are listed and nothing is focused.
  - kitty: `kitten @ --to <address> focus-window --match id:<window id>`. focus-window
    also switches to the OS window that holds it (kitty source).
  - tmux: `tmux [-S <socket>] select-window -t <pane>`, then `select-pane -t <pane>`,
    then `switch-client -t <pane>`. A bare %N target is resolved as a pane id whatever
    the command expects (tmux cmd-find.c), so each of the three finds the session,
    window and pane from it. Outside tmux there is no client to switch, so a failed
    switch-client is said and is not an error: the window and pane stay selected.
  - ide: prints the IDE and the folder. terminal: prints the folder and the account.
    Both exit 0, the output being the answer. A failed kitty or tmux call exits 1 with
    its first stderr line.
  - Refused without any call: a kitty id that is not a positive number, an address that
    does not start with unix: or tcp:, a pane that is not % and digits.

### Needs-you

- D17. Package internal/notify. macOS: `osascript -e 'on run argv' -e 'display
  notification (item 3 of argv) with title (item 1 of argv) subtitle (item 2 of argv)'
  -e 'end run' clalt <title> <text>`. Title and text are arguments of the script and
  never part of its source. The first argument is a constant, so no argument can be read
  as an option, and a text that starts with a dash gets a leading space. Linux:
  `notify-send --app-name=clalt -- <title> <body>` when it is on PATH (`--` ends the
  options). notify-send runs the body through g_strcompress, so backslashes are doubled,
  and notification servers read markup, so & < > are escaped in the body. Elsewhere:
  nothing, and the outcome says so.
- D18. The text is cleaned (control characters out, one line, cut to 120 characters) and
  holds only the account label, the reason claude gave and the last element of the
  folder. No transcript text, no command.
- D19. Rate limit: one empty file per session, `<clalt dir>/notify/waiting/<handle>`,
  made with O_EXCL. NeedsYou sends only when it makes the file and EndWait removes it.
  A file older than 6 hours counts as gone, so a lost EndWait cannot silence a session
  for good. Why a file: the hook handler is a short process per event, so the limit has
  to live on disk, and creating the file is the atomic check and set.
- D20. Off setting: `<clalt dir>/notify/settings.json`, `{"enabled": false}`. No file, or
  one that cannot be read, means on, and `clalt notify status` says when it could not
  be read. `clalt notify [on|off|status|test]`, status by default. `test` sends one
  notification now, ignoring the setting and the limit. Both are additions to the
  brief: a setting needs a switch, and `test` is how the macOS question below gets
  answered on the user's machine.

### Code layout and test seams

- D21. New packages: internal/runcmd (the injected runner: `Run(ctx, name, args...)
  ([]byte, error)` and `Has(name) bool`, with `Exec` for real and `Fake` for tests),
  internal/conversation, internal/terminal, internal/notify. kitty, tmux, osascript and
  notify-send only ever run through runcmd.Runner. In cli: cmd_resume*.go,
  cmd_launch*.go, cmd_jump*.go, cmd_notify*.go and extras.go.
- D22. cli reaches the outside through one struct (extras.go): runner, environment
  lookup, executable path, running sessions, last account. App gets one field for it,
  nil meaning the real ones, and tests set fakes. app.go changes by one line and
  cmd_run.go by none. The wiring pass replaces two defaults.
- D23. Windows: everything builds and passes `GOOS=windows go vet`. kitty and tmux are
  not used there and the notifier is "none".

## Verified, and where

Read in docs or source, not run:
- claude: `--resume <id>` searches the current project and its worktrees, then every
  other project, since 2.1.223; transcripts are
  `~/.claude/projects/<project>/<session-id>.jsonl`; the same session open in two
  terminals interleaves one transcript; names and generated titles are what the picker
  shows (code.claude.com cli-reference and sessions).
- Notification hook: input has session_id, transcript_path, cwd, hook_event_name,
  message, an optional title and notification_type (permission_prompt, idle_prompt,
  elicitation_dialog and others). permission_prompt fires after about six seconds of
  waiting and idle_prompt about 60 seconds after claude finished (code.claude.com hooks).
- tmux: multi-argument shell-command runs without a shell, since 2.0 (tmux CHANGES and
  tmux.1); `new-window -P -c`; `-S socket-path`; `switch-client -t` may name a pane;
  cmd-find.c resolves a bare %N as a pane id for any target type.
- kitty: `kitten @ --to <address>`, KITTY_LISTEN_ON is the default address, `launch
  --type=tab --cwd` prints the new window id, `focus-window --match id:N` switches the
  OS window if needed (kitty docs and rc/*.py).
- osascript(1): arguments after the script go to the run handler.
- notify-send.c: positional summary and optional body, body through g_strcompress.
- Transcripts on this Mac, record types and key names only, counts, no text: see D1 to D5.

## Not verified

- Nothing ran against a real kitty, tmux, osascript or notify-send. The brief forbids it
  here and tmux is not installed. Argument lists are checked against the docs and source
  above, and by fakes in tests.
- macOS shows an osascript notification as coming from Script Editor, from what is known
  of it. Whether this Mac shows it, or needs a switch in System Settings, is open until
  the user runs `clalt notify test` once.
- How osascript reads an argument that starts with a dash is not documented. The
  constant first argument and the leading space keep the question from coming up.
- That the kitty remote-control socket is enabled in the user's kitty. Without it the
  fallback prints the command.

## PR plan

One PR, feat/session-extras into develop, in this order. Every behaviour commit has its
test written first and seen failing.

1. this plan
2. runcmd: runner, Exec, Fake
3. command table entries, extras seam
4. conversation package
5. `clalt resume`
6. terminal package and `clalt launch`
7. `clalt jump`
8. notify package
9. `clalt notify`
10. docs/extras.md, docs/commands.md, README, CHANGELOG

## Wiring pass (after 3a merges, its own PR)

- Adapter from 3a's registry to `sessions`, and from its feed to `lastAccount`.
- In the hook handler: `NeedsYou` on a Notification that means needs-you, `EndWait` on
  every other event of the session.
- Step 4's picker where resume has no account.
- Dashboard actions that call launch, jump and resume.

## Risks

- chdir and PWD in resume are process-wide, restored when cmdRun returns. Fine for a
  command that ends with the session, and covered by a test.
- Record types are undocumented and change between versions (docs say so). An unknown
  type is ignored, so the worst case is a first-prompt title.
- Candidates are picked by mtime and shown by last activity (D7). A file whose mtime was
  changed by something else (a restore, a sync tool) can rank differently from its real
  activity, and one near the cut of `--limit` can be missed. `--limit 0` is exact.
- A transcript that a live session is writing can end in a half line. It is skipped like
  any malformed line.

## Open questions for the user

1. About a third of the transcript lines here are sdk-cli sessions (headless runs). claude's own picker hides them; the brief says every conversation, so this
   lists them and carries entrypoint in the JSON. Hide them by default?
2. tmux before kitty when both are set (D13). Right?
3. Resume with no known account is an error until step 4's picker (D10). Enough?
4. Does 3a's feed keep ended sessions, and for how long? If not, `lastAccount` needs
   another source, such as the usage ledger.
5. Keep the additions: the running-session guard (D12), `clalt notify` and `test` (D20),
   the sign-in check before a tab opens (D15, one claude status call, about a second).
