# clalt v2 step 2: thin profiles

Approved 2026-10-22. Implements step 2 of docs/plans/clalt-v2.md. The rollback point
is release v0.1.0 (tag on 10a1a50), the last release on the token vault.

## Scope (confirmed)

1. `clalt login <name>` creates ~/.config/clalt/accounts/<id>/ and runs claude's own
   `claude auth login` in it. It records which email signed in, and when.
2. `clalt run <name> [-- claude args]` starts claude in that folder. It strips auth
   env vars first (and says which ones, never their values) and checks the folder is
   signed in as the recorded email.
3. `clalt migrate [--dry-run]` copies old history from
   ~/.config/clalt/profiles/<name>/projects into ~/.claude/projects. It never
   overwrites and never touches the old folders.
4. `clalt logout <name>` signs out and keeps history. `clalt remove <name>` signs out
   and hides the account. `clalt rename` changes the label only. `clalt list` and
   `clalt whoami` show each account's sign-in state.
5. Retired: the token vault, keepalive and its scheduler, the restart supervisor,
   clalt's own OAuth client, `clalt add`, `clalt refresh`, `clalt export` and
   `clalt import`, and `--live` (it read the vault token; step 3's statusline feed
   brings live limits back). The old dashboard drops its token and keepalive panels.
   Vault files stay on disk until the user removes them.
6. `clalt doctor` checks the claude version, each account's sign-in and identity,
   the profile links, MCP sync, and leftovers of the old design.

## Decisions

### Account folders

- Each account is ~/.config/clalt/accounts/<id>, used as CLAUDE_CONFIG_DIR. claude
  keys the login to that path, so the folder never moves.
- The id is the name given at creation. Existing accounts keep their current names
  as ids. If an id is taken (an account was renamed away from it), the new id gets a
  numeric suffix (robin-2).
- The label is the display name and the CLI handle. Rename changes only the label.
- Ids and labels pass account.ValidateName and are unique case-insensitively (macOS
  file systems are case-insensitive, so robin and Robin would share a folder).
- Removed accounts are hidden, not deleted. `clalt login <label>` brings one back in
  the same folder.
- Every id that becomes a folder name goes through one check (config.AccountDirFor):
  a single local path element, so not empty, not . or .., no separator, no NUL and no
  Windows device name. A legacy id that passes it keeps working even if
  account.ValidateName would reject it today.

### What a profile shares with ~/.claude

The shared home is ~/.claude. If CLAUDE_CONFIG_DIR is set and points outside
clalt's own dir, that is the shared home instead, and its .claude.json is the shared
global config. A CLAUDE_CONFIG_DIR inside clalt's dir (a `clalt run` session) is
never treated as the shared home.

- Folders are linked by default, so new folders the user adds show up in every
  account on the next run. Kept per account (never linked): sessions, daemon, jobs,
  backups, cache, statsig, telemetry, downloads.
- skills and plugins are split, not linked whole. claude.ai sessions download that
  account's own skills and plugins into skills/synced and plugins/synced, and its
  marketplaces into plugins/known_marketplaces_claudeai.json, so a link to the whole
  folder would write one account's downloads into the shared folder. The account gets
  a real skills/ and plugins/ folder instead, and every entry of the shared one (files
  and folders alike) is linked into it, except these, which stay per account: skills:
  synced, .trash; plugins: synced, known_marketplaces_claudeai.json, .trash. A skills
  or plugins link from an earlier layout is replaced by a real folder with links
  inside. Only the link goes, so nothing is lost. When the shared home no longer has
  the folder at all, the links the account made into it are removed and what the
  account owns stays. A shared folder that is listed but unreachable removes nothing.
  Inside a split folder these rules apply: every entry of the shared folder is linked,
  files and folders alike, except the names that stay per account and
  .credentials.json and .claude.json; a real file, or a real folder with anything in
  it, where a link belongs is kept and reported; a link clalt made for a name that left
  the shared folder is removed, and a link aimed elsewhere is left alone; an entry that
  cannot be read is kept and reported. The loose-file allow-list and the list of
  per-account folders do not apply there.
- Loose files are linked only from an allow-list: settings.json, settings.local.json,
  CLAUDE.md, keybindings.json, history.jsonl. claude keeps per-account state in loose
  files (signed org policy and managed settings, connector sign-in cache, daemon
  status, stats), so unknown files stay per account.
- Never linked whatever happens: .credentials.json and .claude.json.
- Sync runs on every login and run, one at a time per account (a lock file in the
  account folder). A missing link is created; a link pointing somewhere else is
  re-pointed, atomically: the new link is made under a temporary name and renamed over
  the old one. Windows cannot rename over a link to a folder, so there the old link is
  removed first and the name is absent for a moment. A link whose source is gone is
  removed. A real file where a
  link belongs is kept and reported (claude may replace a link with a file when it
  saves), and so is a real folder with anything in it; an empty real folder holds
  nothing, so the link replaces it. Nothing is ever copied in place of a link.
- Gone means absent from the shared home, and nothing else. An entry that is listed
  but unreachable is never taken for gone: the account's link for it is not removed.
  An entry that cannot be read (permissions, a link loop) is reported. A link in the
  shared home whose target is missing (a volume that is not mounted) is reported
  only where the account has a link to it, so a dead link nobody uses stays quiet.
- If the shared home does not exist at all, sync stops with an error and removes
  nothing: the home may only be unmounted.
- Links use absolute targets. Sync refuses an empty or relative home or account
  folder, and an account folder that is the shared home.
- Link failures (Windows without Developer Mode) are reported by name. Windows stays
  best-effort.

### MCP servers

User-scope servers (top-level mcpServers in the shared .claude.json) are mirrored into
the account's .claude.json on every run:
- a server in the shared config is added or updated in the account;
- a server clalt synced before that is gone from the shared config is removed;
- a server the account added itself is kept.

clalt remembers which names it synced in a small file in the account folder. Every
other key in the account's .claude.json is preserved verbatim. The file is written
atomically, only when something changed, with mode 0600, and never through a symlink.
Server definitions are never printed (env blocks can hold secrets); only names are.

- If the shared .claude.json is missing while servers were synced from it before, sync
  stops with an error and changes nothing: a missing file is more likely an unmounted
  home than a decision to drop every server. A missing file with nothing synced before
  is just no servers.
- A shared server that replaces a server the account added itself under the same name
  is reported as overwritten, so the caller can say so.
- The sync holds the account's lock throughout. It reads the account's .claude.json
  again right before writing, and if claude changed it meanwhile it plans again, up to
  three times, then gives up with an error.

### Identity

- `claude auth status` prints JSON and exits 0 when signed in, 1 when not (docs:
  cli-reference). Keys seen on 2.1.303: loggedIn, authMethod, apiProvider, email,
  orgId, orgName, subscriptionType, configDirectory, projectsDirectory,
  analyticsDisabled. A signed-out folder reports loggedIn false and authMethod none.
- The output must carry loggedIn, authMethod and configDirectory, and the exit code
  must agree with loggedIn (0 exactly when it is true). Anything else is an error.
- A folder counts as signed in only when loggedIn is true, authMethod is claude.ai,
  apiProvider is firstParty (or absent), email is set, and configDirectory is the
  account folder. Anything else (for example api_key_helper from shared settings, or
  the bedrock provider) is reported with a typed reason. claude reports loggedIn true
  with a null email for an expired or incomplete credential, and that counts as not
  signed in.
- After `claude auth login`, clalt reads the status for the folder and records
  email, org and plan with signedInAt. Signing an existing account in again passes
  `--email <recorded email>` so the browser form is pre-filled.
- Login refuses, and signs the folder out again, when the email already belongs to
  another account, or differs from the email recorded for this account. The message
  says to use a private browser window and try again. Before login, clalt prints that
  tip.
- Run refuses when the folder is not signed in, or is signed in as a different email
  than recorded, and prints the exact command that fixes it.
- Logout keeps the recorded email, so the next login is still checked. Remove clears
  it, so a later login records a fresh identity.
- If sign-out fails, remove stops and does not hide the account, so no login is left
  behind in a hidden folder.
- A login is due 30 days after signedInAt (claude's refresh grants are capped there).

### Auth env vars

Before every claude call (login, run, status, logout) clalt removes the env vars that
would override the folder's own login, and prints their names:
CLAUDE_CODE_OAUTH_TOKEN, CLAUDE_CODE_OAUTH_REFRESH_TOKEN, CLAUDE_CODE_OAUTH_SCOPES,
ANTHROPIC_API_KEY, ANTHROPIC_AUTH_TOKEN, ANTHROPIC_PROFILE,
ANTHROPIC_FEDERATION_RULE_ID, ANTHROPIC_ORGANIZATION_ID,
CLAUDE_SECURESTORAGE_CONFIG_DIR, CLAUDE_CODE_USE_BEDROCK, CLAUDE_CODE_USE_VERTEX,
CLAUDE_CODE_USE_FOUNDRY, CLAUDE_CODE_USE_MANTLE, CLAUDE_CODE_USE_ANTHROPIC_AWS,
ANTHROPIC_AWS_API_KEY, ANTHROPIC_BASE_URL, ANTHROPIC_CUSTOM_HEADERS. The first twelve
follow the precedence list in the authentication docs: everything there outranks a
subscription login. The next three are provider and credential switches from the
env-vars docs, and the last two could send the account's token to another host.
CLAUDE_CONFIG_DIR is always set to the account folder (except for the version check,
which is not about any folder). An apiKeyHelper, or an env block setting one of these
vars, in the shared settings would override every account's login; doctor warns
about both, and the status check above catches it at run time.

### Calling claude

- The binary is CLALT_CLAUDE_BIN or claude, resolved with exec.LookPath before use.
- Minimum claude version is 2.1.301. Earlier versions could sign a folder out when
  `claude auth status` exited in the middle of a token refresh (changelog 2.1.301),
  and clalt runs that command often.
- The version is checked before the first status, login or logout of a process, by
  running `claude --version` (about 10 ms) with the auth variables stripped. A claude
  that is too old, or whose version cannot be read, is an error that names the
  versions.
- Status, login and logout refuse an empty, relative or missing account folder (claude
  makes the folder it is given and writes .claude.json, a lock dir and backups/ in it)
  and refuse the shared home itself. Logout asks for the status afterwards to confirm
  the folder is signed out. If the call was cut short, or the check could not be made,
  the error says the sign-out state is unknown.
- Cancelling a claude auth call sends an interrupt and kills the process only after a
  30 second grace period (exec.Cmd Cancel and WaitDelay). Windows cannot send an
  interrupt, so there nothing is sent and the grace period simply runs first. The
  error says whether claude stopped after the interrupt or was killed after the grace.
  This does not protect a token refresh by itself: real claude 2.1.303 has no SIGINT
  handler in `auth status`, so an interrupt ends it at once, and the grace period only
  helps a claude that handles the signal. Ending claude in the middle of a token
  refresh can sign the folder out, so callers must not use short timeouts on auth
  calls. Status, Logout and Version get a 2 minute timeout when the caller sets none.

### state.json

Additive only. The map key stays and is now the id. Meta gains label, org,
signedInAt and hidden. Existing fields are kept and still written, so a v0.1.0
binary can read the file. (v0.1.0 rewriting the file would drop the new fields; that
is accepted for an emergency rollback.) The active pointer is no longer used.

Parallel `clalt run` sessions write the file, so every update runs under a lock file
(state.json.lock) and the write is atomic. An empty state.json is an error, not "no
accounts", so a file that something cut short is never written over.

### Retired commands and flags

Removed commands (add, refresh, keepalive, export, import) and flags (run --refresh,
--isolated, --no-supervise, --link; usage and dashboard --live; doctor --fix) exit 2
with one line saying what replaces them. The switch log stops growing, so new usage
shows as unattributed until step 3 adds per-session attribution.

### Doctor

- claude on PATH and at least 2.1.301 (see Calling claude).
- Per account: folder present, links in sync, MCP in sync, signed in, signed in as
  the recorded email, login due within 3 days.
- Auth env vars set in the current shell (names only), apiKeyHelper in shared
  settings.
- Leftovers, reported and never deleted: the old vault, old profiles with uncopied
  history (points at `clalt migrate`), an old keepalive job still installed, and
  ~/.claude/.credentials.json written by the old switch flow.

## PRs

1. feat/profile-core: building blocks, nothing wired yet. config paths for accounts
   and the shared home; profile link sync and its read-only health check; MCP sync;
   internal/claudecli (auth status, login, logout, version, env stripping); account
   Meta fields and metadata-only store methods (create with id allocation, resolve by
   label, set identity, clear sign-in, hide, unhide, relabel, touch).
2. feat/history-migrate: `clalt migrate`. Built in parallel with 1. Cuts the
   CHANGELOG [0.1.0] section if it lands first.
3. feat/thin-profiles, stacked on 1: the CLI on profiles (login, run, logout, remove,
   rename, list, whoami, sessions and budget by label), the retirement, doctor,
   README and CHANGELOG.

Every PR is test-first. Tests use temp HOME and XDG dirs and a fake claude binary
(CLALT_CLAUDE_BIN); they never read the real ~/.claude or ~/.config/clalt.

## DECIDE coverage

Applied: 1 (scope), 2 (layout, sharing, MCP, state), 3 (identity, env, paths,
remove), 4 (package fit), 6 (CLI surface), 7 (additive data, copy-only migration),
9 (rollback via v0.1.0), 10 (doctor). Skipped: 5 (no events), 8 (CLI text only),
16 and 17 (local CLI, no staging or flags), 19 (no incident).

## Risks and open assumptions

- Which files claude saves by replacing them (breaking a link) is unverified. Sync
  keeps such a file and reports it, so nothing is lost, but that account then has
  its own copy until the user resolves it.
- MCP sync writes an account's .claude.json while another session of the same
  account may be running. Writes are atomic and only on change; the small race is
  accepted.
- The per-account folder list comes from what claude 2.1.296 to 2.1.303 wrote on
  this Mac, not from docs. Unknown loose files stay per account by design.
- `claude auth status --json` keys follow the docs at the time of writing; the
  parser tolerates unknown keys and fails loudly on anything that is not JSON.
