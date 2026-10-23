# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Session registry and feed. `clalt run` gives each session a handle
  (`CLALT_SESSION`), keeps a record of it with the terminal location, and passes
  claude a `--settings` that installs hooks and a status line, which write the
  state of each session and the 5-hour and 7-day limits of each account to
  `~/.config/clalt/feed/`. Your own settings are never modified, and your own
  hooks and status line command keep running. `clalt sessions --json` gains
  `handle`, `folder`, `wrapperPid`, `claudeSessionId` and `terminal`, and
  `clalt doctor` checks the feed. See docs/feed.md.

- `clalt migrate [--dry-run]`: copies the conversation history held in
  per-account profile folders (`~/.config/clalt/profiles/<name>/projects`) into
  the shared projects folder, which is `~/.claude/projects`, or
  `$CLAUDE_CONFIG_DIR/projects` when that is set to a folder outside clalt's
  own. It never overwrites a file, never follows a link and leaves the profile
  folders as they were, so it is safe to run again. A file that exists with
  different content is listed as a conflict with the reason, and the profile's
  version stays only in the profile folder. A dry run reports what a real run
  would do and writes nothing.
- `clalt login <name>` signs an account in through claude's own `claude auth
  login`, in the account's own folder (`~/.config/clalt/accounts/<id>`), and
  records the email, plan and organization that signed in, and when. Signing in
  again passes the recorded email so the browser form is pre-filled. It refuses,
  and signs the folder out again, when the email already belongs to another
  account or differs from the recorded one, names the email that signed in, and
  tells you to use a private browser window. A new name makes the account; a
  removed one comes back once the sign-in works. When the first sign-in of a new
  name is refused or fails and nothing is left signed in, the account is hidden
  again instead of being left in the list, and trying the same name again reuses
  it. When claude may have left a login in the folder (it could not be asked, or
  an `apiKeyHelper` overrides it), the account stays in the list, or comes back
  into it if it was removed, and the message says that `clalt logout <name>`
  signs it out. When the account's history does not reach the shared history,
  it prints one warning line right after linking the account and goes on with
  the sign-in: `run` will refuse the account until it does. The tip to use a
  private window for the sign-in goes to stderr, so stdout holds only what claude
  and the result of the sign-in say.
- `clalt run <name> [-- claude args]` starts claude in the account's folder. It
  links the folder to the shared claude home, mirrors user-scope MCP servers,
  removes the variables that would override the login (naming them, never their
  values) and refuses unless claude reports a claude.ai login in that folder as
  the recorded email, printing the command that fixes it. When something outside
  the account overrides the login, such as an `apiKeyHelper` in the shared
  settings, it points at `clalt doctor` instead. Just before it starts claude it
  checks that `projects` in the account folder is a link that reaches the shared
  history folder, and refuses, pointing at `clalt doctor`, when the account has
  a real `projects` folder or the shared one is not reachable (an unmounted
  volume), so that claude never makes a real `projects` folder and forks the
  history. It stops with a message when the shared claude home does not exist,
  and never makes it: the home may only be unmounted. A home that exists
  without a `projects` folder gets one (mode 0700) from `login` and `run`. What
  clalt says while it prepares the session (what it synced, the variables it
  does not pass on, what it left alone) goes to stderr, so stdout is claude's
  alone. While the session runs clalt waits for claude: at a terminal a Ctrl-C
  is claude's to handle, and a SIGTERM or SIGHUP sent to clalt is passed on to
  claude, and so is a SIGINT when no terminal sends it a Ctrl-C (its input is
  not a terminal and it is not in the foreground group of one: a headless `-p`
  run, a script, a supervisor), unless the signal was ignored when clalt
  started (under `nohup`, for example). clalt exits with claude's own exit
  code.
- sharing with `~/.claude`: folders are linked, except the ones claude keeps per
  account. `skills` and `plugins` are linked entry by entry into a real folder of
  the account, so what claude.ai downloads for one account (`synced`, `.trash`,
  `plugins/known_marketplaces_claudeai.json`) stays with that account. Loose
  files are linked only from an allow-list. A link that points elsewhere is
  re-pointed without a moment where the name does not exist (on Windows a link
  to a folder cannot be replaced that way: it is removed and made again, and
  the name is missing until the next sync if that fails), an empty folder in
  the way is replaced by the link, and a real file or a folder with content is
  kept and reported with the reason, never copied over. An entry that cannot be
  read, or a link to something that is missing (a volume that is not mounted),
  is kept and never taken for gone. When the shared `skills` or `plugins` is gone
  altogether, only the account's links into it are removed, and credential files
  are never linked out of them. One sync runs at a time per account. A shared
  MCP server that replaces one the account added itself is reported by name, and
  a shared config that is gone while servers were mirrored from it stops the sync
  with a warning instead of dropping the servers.
- `clalt logout <name>` signs the folder out and keeps the history and the
  recorded email. It works on a removed account too, and leaves it removed.
  `clalt remove <name> [--force]` signs out unless claude says plainly that
  nobody is signed in, and hides the account; `clalt login <name>` brings it
  back in the same folder. Without `--force` it asks, and when nobody can answer
  (the input is closed) it exits 1 and says to pass `--force`. When claude
  cannot say whether a sign-out worked, both say the state is unknown and
  change nothing they keep. When the sign-out worked but `state.json` could not
  be written, they say so and that running the command again tries again.
- `clalt list` shows each visible account with its live sign-in state (asked
  from claude a few at a time, `unknown` with a reason when that fails), plan,
  the date to sign in again by (30 days after the sign-in) and last use.
  `clalt whoami` shows the account of the current shell, an old v0.1.0 profile
  session, or that the shell is in no clalt account. Both keep `--json`, with
  the new shape described under Changed.
- `clalt doctor` checks claude (on PATH, at least 2.1.301), each account (folder,
  links, MCP servers, signed in, signed in as the recorded email, login due
  within three days, and a warning when claude said something about the
  account, such as a corrupt `.claude.json` it set aside), the folders of
  removed accounts that are still signed in, that `state.json` can be read,
  auth variables set in the shell or in the shared `settings.json` and
  `settings.local.json` (names only), and leftovers of the old
  design: the token vault, the old profile folders (a warning for as long as one
  exists, naming any plaintext `.credentials.json` v0.1.0 staged in it, and
  saying whether its history is copied), the old keepalive job and its marker
  files, and on macOS a `~/.claude/.credentials.json`, which the old switch flow
  may have written but claude also writes when it cannot save its login in the
  keychain, so doctor says how to tell before anything is deleted. It only
  reports, and keeps `--json` with the new shape described under Changed. It
  judges the links the way `run` is going to find them: a shared `projects`
  folder that is missing, or an empty `projects` folder in an account, is a
  warning, since `run` makes the one and replaces the other with the link. It
  exits 1 only for what is really broken: claude missing or too old, an id that
  cannot be a folder name, two accounts that share a folder, a `projects` folder
  in an account that has something in it, a `state.json` that cannot be read,
  and a login of a different email than the one recorded. Everything you fix by
  doing something, such as signing in or mounting a volume, is a warning (a
  shared history folder that is not reachable is one), so doctor after a plain
  logout, or right after upgrading from v0.1.0, exits 0.
- `clalt doctor` reports accounts whose ids differ only in case and so share one
  folder on a file system that ignores case (macOS and Windows by default), and
  `login` and `run` refuse them until one of the two is removed. v0.1.0 allowed
  such names.
- `clalt usage` reports usage that cannot be placed under an account as
  `unattributed` instead of folding it into the account that was last active.
  That includes everything from the first sign-in made with this version on,
  which the old switch log knows nothing about: `state.json` keeps the time of
  that sign-in as `thinSince`, and that usage counts against no budget.
  `usage`, `budget` and the dashboard read the transcripts of the shared claude
  home, also inside a `clalt run` session, and `usage` and `budget` exit 1 and
  say why when they cannot read them, instead of reporting zero. `unattributed`
  is a reserved label, in any case, so that no account is taken for that row:
  `login` and `rename` refuse it.

- `clalt rules <name>` shows and edits per-account rules, kept under a `rules`
  key of `state.json` (v0.1.0 ignores it): the folders the account may serve (a
  rule matches the folder and everything under it, links followed, case ignored
  where the file system ignores it; no rules means any folder) and whether
  auto-switch may use the account. Flags: `--add-folder`, `--remove-folder`,
  `--auto-switch`, `--no-auto-switch`, `--json`. `clalt doctor` warns about a rule
  for a folder that is gone or that is not an absolute path.
- `clalt run` with no name (or `clalt run -- <claude args>`) picks the allowed
  account with the most room, 100 minus the higher of the 5-hour and 7-day
  figures from the feed, and says on stderr which and why. Accounts without
  recent limit data rank after those with data, then by label. Until the feed is
  wired in every account ranks by label.
- `clalt move <session> <account>` asks the wrapper of a running session to move
  it to another account at its next pause, with `claude --resume`, after the same
  link sync and sign-in check as `run`. The wrapper ends claude with SIGTERM,
  restores the terminal and prints one line. It waits for the session feed
  (step 3a): until then `move` says it cannot see sessions.
- Auto-switch, off unless turned on per account, moves an idle session whose
  5-hour window reached 90 percent, or whose claude reported the limit, to an
  allowed account with auto-switch on and at least 20 points more room, never
  within 30 minutes of a move. It is meant for accounts you are entitled to use
  for the work at hand, such as your own personal and work accounts, and each
  account's terms apply. See docs/move.md.

### Changed

- `clalt rename` changes the label only: the account keeps its id and folder,
  because claude keys the login to the folder. Accounts are named by their label,
  ignoring case, on every command line.
- `clalt remove` signs out and hides the account instead of deleting a vault
  slot. The folder and its history stay.
- The JSON of `clalt list`, `whoami` and `doctor` changed, and not in a way an
  old script survives. `list --json` gave `name`, `active`, `email`, `plan`,
  `addedAt`, `lastUsedAt` and `expiresAt`; it gives `id`, `label`, `email`,
  `org`, `plan`, `signedIn` (true, false or null), `reason`, `signedInAt`,
  `loginDueAt`, `lastUsedAt` and `folder`, and `name`, `active`, `addedAt` and
  `expiresAt` are gone. `whoami --json` gave the same object, or
  `{"active": null}`; it gives a `kind` (`account`, `oldProfile`,
  `removedAccount`, `unknownAccount` or `none`) and, for an account, the fields
  of `list`. `doctor --json` is not new, but its shape is: it gave `active`,
  `loginFilePresent`, `nearExpiry`, `orphanedSecrets`, `metadataWithoutSecret`
  and `fixed`, and it gives `{checks, failed, warned}` now, with a `scope`,
  `check`, `status` and `detail` for each check.
- `clalt sessions`, `usage` and `budget` take and show labels. `sessions --json`
  has `account` (the id) and `label`; `usage --json` has `id` and `name` (the
  label) and no longer has `active`, `expiresAt`, `live` or `liveNote`, and the
  row for usage that nobody can be blamed for is `unattributed`, as id and as
  name; `budget --json` gains `id` and prints nothing but the JSON, also right
  after a cap was set (v0.1.0 put a line of text in front of it).
- accounts share the conversation history in `~/.claude/projects` instead of
  keeping a separate history each, along with settings, skills, plugins,
  commands, agents and CLAUDE.md.
- `--json` after the account name of `clalt run`, with or without a `--`, now
  goes to claude instead of being taken by clalt.
- `clalt dashboard` opens the focus-panes dashboard (Bubble Tea v2) on your
  real accounts: sign-in state from `claude auth status` (asked in the
  background, cached, never per redraw), usage per day, project and model from
  the transcripts, budgets, and run, sign in, sign out, rename and budget from
  the keys. Enter and `s` run the code of `clalt run` and `clalt login` with the
  terminal handed over, and a refusal stays on screen until you press enter.
  Limits and running sessions show as not available until the statusline feed
  and session registry are connected. Failures are logged to
  `~/.config/clalt/logs/dashboard.log`. `--demo` still shows made-up data.
- `state.json` gains `label`, `org`, `signedInAt` and `hidden` per account, and
  `thinSince` once. The fields v0.1.0 wrote are still written, so the v0.1.0
  binary can read the file. The `active` pointer is kept and never changed. The
  file is saved atomically under a file lock, and an empty one is reported as an
  error instead of being read as no accounts.
- claude's own commands are called through one wrapper that strips the auth
  variables, checks the claude version once per process, refuses an empty,
  relative or missing account folder and the shared home itself, and reads
  `claude auth status` strictly: an answer that lacks the keys that decide it, or
  whose exit code disagrees with it, is an error.
- each call to claude that is not interactive gets two minutes: `auth status`
  (with the version check it makes first), `--version` on its own, and
  `auth logout`, whose two minutes cover the version check, the sign-out and the
  status check together. At the deadline claude is interrupted, and killed 30
  seconds later if it is still running (on Windows nothing is sent, the 30
  seconds just run before the kill). A shorter timeout would cancel claude,
  which has no interrupt handler, in the middle of a token refresh. `auth login`
  and the session of `clalt run` have no timeout. On macOS and Linux these calls
  start in a process group of their own, so a Ctrl-C at the terminal ends clalt
  and lets them finish; the sign-in and the session stay in the foreground.
- `clalt` no longer creates `vault.key` on every command, `clalt version`
  included.

### Removed

- the token vault, clalt's own OAuth client and token refresh, and everything
  built on them: `clalt add`, `clalt refresh`, `clalt export` and `clalt import`
  (accounts hold no secrets now), `clalt keepalive` and its background job with
  the launchd, systemd and Task Scheduler code, the restart supervisor of
  `clalt run`, and `--live` on `usage` and `dashboard` (live limits return with
  the statusline feed). Old vault, keepalive and marker files stay on disk until
  you delete them; `clalt doctor` lists them.
- `clalt login --magic-link`, `clalt run --refresh`, `--isolated`,
  `--no-supervise` and `--link`, and `clalt doctor --fix`. The removed commands
  and flags exit 2 with one line that says what replaces them.
- the environment variables of the old OAuth login and keepalive, which nothing
  reads any more: `CLALT_NO_BROWSER`, `CLALT_OAUTH_CLIENT_ID`,
  `CLALT_OAUTH_SCOPES`, `CLALT_AUTHORIZE_URL`, `CLALT_TOKEN_URL`,
  `CLALT_REDIRECT_URI`, `CLALT_API_BASE` and `CLALT_NO_AUTO_KEEPALIVE`.
  `CLALT_CLAUDE_BIN` and `CLALT_ASCII` still work.
- the `golang.org/x/crypto` dependency.
- the old dashboard (`internal/tui`) and the Charm v1 libraries it used.

### Fixed

- every command waited about 5 seconds on a terminal that does not answer a
  colour query: bubbletea v1 asked for the background colour when the program
  started. It is gone with the old dashboard.
- `go test` hung when stdout was a terminal, because the dashboard asked the
  process's stdout and not the app's own streams.

### Security

- clalt stores and reads no tokens. Each login stays where claude keeps it,
  keyed to the account's folder: an entry in the Keychain on macOS, and a
  `.credentials.json` in the folder only on Linux and Windows. clalt never reads
  a credential file or the keychain.
- the variables that would override an account's login (`ANTHROPIC_API_KEY`,
  `CLAUDE_CODE_OAUTH_TOKEN`, the provider switches, `ANTHROPIC_BASE_URL` and
  `ANTHROPIC_CUSTOM_HEADERS`, and the rest of the list in docs/auth.md) are
  removed before every claude call, and `clalt doctor` names the ones set in the
  shell or in the shared settings, never their values.
- an account id is checked before it is used as a folder name: one path element,
  not empty, not `.` or `..`, no separator, no NUL byte, no Windows device name.
  An id from an older state file that passes keeps working even when it would not
  be accepted as a new name, and one that does not pass cannot point a folder
  outside the accounts directory.
- text that claude reports (email, plan, organization, error lines), what a
  `state.json` that an older clalt wrote holds (labels, which default to the id,
  email, plan, organization) and names found on disk or in transcripts (file
  names, the model id and project of a transcript, the folder of a session,
  `CLAUDE_CONFIG_DIR`) have control characters, line separators and
  bidirectional overrides shown as escapes (`\x1b`) before they are stored or
  printed, so an organization name cannot put terminal escape sequences on the
  screen. The zero width joiners that scripts and emoji need are kept.
- an email that claude does not report never matches the recorded one, and an
  empty email is not equal to another empty one.

## [0.1.0] - 2026-10-22

### Added

- multi-account vault: named credential slots stored in a local encrypted
  vault (AES-256-GCM), replacing the macOS Keychain entirely.
- keychain-free `clalt run`: launches claude for a named account without
  swapping a global login, via `CLAUDE_CODE_OAUTH_TOKEN` on macOS and an
  isolated per-profile credentials file on Linux/Windows, so different
  accounts can run in parallel.
- supervisor mode for `clalt run` on macOS: refreshes the account's token
  ahead of expiry and transparently restarts claude with `--resume` so a
  session survives past a single token's lifetime.
- `clalt login --magic-link`: sign into someone else's account via a
  claude.ai magic link before the OAuth consent screen.
- persistent multi-panel dashboard TUI (`clalt dashboard`): accounts,
  sessions, and keepalive panels beside a detail pane, with filtering, a
  command palette, and in-place account management.
- `clalt dashboard --demo`: a preview of the new focus-panes dashboard
  (Bubble Tea v2) running on demo data. The current dashboard stays the
  default, and the preview never reads or changes real accounts. (#6)
- background keepalive: automatic scheduled token refresh via the native OS
  scheduler (launchd, systemd user timer, Task Scheduler).
- `clalt usage`: local and `--live` usage stats, cost estimates, and
  transcript-based aggregation. Live responses are cached on disk to respect
  the endpoint's rate limit.
- per-account spend caps (`clalt budget`, `--daily`/`--weekly`).
- `clalt export` / `clalt import`: encrypted backup and restore of all
  accounts.
- `clalt version` (and `--version` / `-v`): prints build version, with
  `--json` for machine-readable output.
- internal/fsutil (atomic file writes) and internal/fslock (cross-process
  file locks): shared helpers that nothing calls yet. (#9)
- internal/usage can dedup transcript records by message id and request id,
  so a reply written as several transcript lines counts once. (#10)

### Fixed

- fixed a data race between `clalt run` status output and the child's output
  copy when output is not a terminal. (#8)
- cost estimates matched model ids by substring in random map order, so one
  model could be priced differently from run to run. Pricing now walks an
  ordered table, most specific model id first. Prices for newer models are
  updated (opus 4.5 and later, sonnet 5, fable 5, mythos 5.1, and haiku 5
  with a higher rate above a 100,000-token prompt), so estimates change.
  (#10)

### Security

- account names are validated (letters, digits, `.`, `_`, `-`, at most 64
  characters, no Windows device names) when an account is created or renamed
  and before `clalt run` stages a profile, so a name like `../../x` can no
  longer become a filesystem path. An existing account with an older,
  invalid name must be renamed before it can run. (#11)
- removed the legacy macOS Keychain migration path and the
  github.com/keybase/go-keychain dependency; clalt no longer touches any OS
  keychain, enabling pure-Go cross builds (CGO_ENABLED=0) on every platform.
- bumped the Go toolchain to 1.26.6, fixing four stdlib vulnerabilities
  (net/url, crypto/tls, encoding/asn1, net/http via golang.org/x/net/idna)
  flagged by govulncheck.
- bumped the Go toolchain again to 1.26.10, fixing ten more stdlib
  vulnerabilities (net/http, mime/multipart, crypto/tls, crypto/x509)
  flagged by govulncheck.
