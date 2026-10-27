# Plain claude beside clalt: the default home

Problem: people run plain `claude` (config in ~/.claude, its own login) next to
`clalt run <name>`. clalt only sees clalt run sessions, so the dashboard shows no
limits for an account whose email is also signed in to ~/.claude, and the usage of
plain claude is "unattributed" even though it ran as that same email.

## Scope

1. Recognise the default home by email and show it.
2. Attribute usage of sessions that clalt did not start to the matched account,
   and record that in the ledger.
3. An opt-in live feed for plain claude: `clalt feed install | uninstall | status`.

Not in scope: changing what plain claude does, touching its login, any claude
call on a hot path (hooks, statusline, list, dashboard redraw).

## 1. Matching

`claudecli.StatusOfShared` asks claude for the default home exactly as plain
claude sees it (CLAUDE_CONFIG_DIR unset, auth variables stripped, no short
timeout). The answer is cached in `~/.config/clalt/default-home.json`:

```go
type Cache struct {
    State     string    `json:"state"`             // matched, outside, unusable
    Account   string    `json:"account,omitempty"` // id of the matched account
    Email     string    `json:"email,omitempty"`
    Reason    string    `json:"reason,omitempty"`  // words, for doctor
    Off       bool      `json:"attributeOff,omitempty"` // usage attribution undone
    CheckedAt time.Time `json:"checkedAt"`
}
```

Rule (package internal/defaulthome, a pure function): the status must be a
claude.ai login with an email in the first party provider (claudecli SignedIn
against the folder it reported). Then the visible account whose recorded email
equals it, case-insensitively, is the match. Other outcomes:

- matched: one visible account has that email.
- outside: signed in, but nobody has that email, or the login is not claude.ai
  (api key, third party provider, no email), or two visible accounts have the
  same email (guessing would charge the wrong one). Plain claude stays outside
  clalt, and the reason says why.
- unusable: claude could not be asked (missing, too old, broken output).

Who refreshes the cache: `clalt feed install`, `clalt feed status`, and
`clalt usage` and the dashboard on start (in the background) when the cache is
missing or older than a day. Everything else only reads it (hooks, statusline,
list, doctor, dashboard draws). Doctor must stay read-only, so it reports from
the cache and says when it was checked, or that it never was. A missing or corrupt cache means "not matched". A refresh that
ends `unusable` keeps the old match (a failed check is not a sign-out) but
records the reason and time.

Shown: `clalt list` prints "<label> is also the default claude (~/.claude)" under
the table (`--json`: field `defaultHome` on the row) and the dashboard Profile has
a "Default" row saying "also the default claude (~/.claude)". Doctor has a line:
the matched account, "outside" with the reason, or unusable, and one for whether
the feed is installed.

## 2. Usage attribution

The ledger is `~/.config/clalt/session-accounts.jsonl`. A line gets an optional
`source`: absent means a clalt run session saw the id (a hook), `"default"` means
attributed to the matched account of the default home:

```json
{"ts":"...","sessionId":"<claude session id>","account":"work","source":"default"}
```

Rules, in `usage.DefaultLines`:

- only records at or after the thinSince cutover (the time before it goes by the
  switch log and is not touched);
- only sessions whose id has no line in the ledger yet. A session with a line is
  never reassigned, and a moved session keeps its split;
- records with no session id stay unattributed;
- `Ts` of the new line is the first record of that session, so the whole session
  falls under it.

The report path appends the new lines (append only) when the cache says matched
and attribution is not off, then attributes as before. The ext hook appends the
line the first time it sees a session id, which is more exact in time. A later
change of the default login does not rewrite old lines: they say what was
believed then.

Show and undo: `clalt feed forget-default` removes exactly the lines with source
`default` from the ledger (atomic rewrite), so that usage is unattributed again,
and sets `attributeOff` in the cache so the next report does not add them back.
`clalt feed remember-default` turns it on again. Documented in feed.md.

## 3. Live feed for plain claude (`clalt feed`)

`clalt feed install [--yes]`, `uninstall [--yes]`, `status [--json]`. The edit
is of `settings.json` in the shared claude home (SharedClaudeDir).

Install:

1. Refresh the match and refuse with a clear message when there is none (the
   feed would have nothing to attribute to). There is no override.
2. Refuse when settings.json is a symlink, not a regular file, not valid JSON or
   not a JSON object. A missing file is allowed and is created.
3. Confirm (prompt on a terminal, `--yes` for scripts; with no terminal and no
   `--yes` it refuses). The prompt says what will change.
4. Back up the current file to `~/.config/clalt/backups/settings/settings-<UTC
   timestamp>.json` (mode 0600). Nothing is written when the backup fails.
5. Merge, touching no other key: for each event of cmd_hook_events.go append one
   matcher group holding clalt's handler to `hooks.<Event>` (the user's groups
   are kept verbatim, in order, theirs first). The handler command is
   `'<abs clalt>' hook <Event> clalt-feed`. The trailing word `clalt-feed` is the
   marker that uninstall and status find handlers by; the hook ignores it.
6. statusLine: when the user has one that is not clalt's, its raw JSON object is
   stored in clalt's own state (`~/.config/clalt/feed-install.json`, never in
   settings.json) and replaced by clalt's statusLine
   (`'<abs clalt>' statusline clalt-feed`, with padding, refreshInterval and
   hideVimModeIndicator copied). `clalt statusline` then runs the stored command:
   readUserStatusLine reads the state when settings.json's own is clalt's.
7. Write through fsutil.WriteFileAtomic, with the mode of the old file (0600 if
   new).
8. Idempotent: an event that already has a handler with the marker gets no second
   one, and a second install with nothing to add makes no backup and says so.

State file `feed-install.json`:

```go
type installState struct {
    InstalledAt time.Time       `json:"installedAt"`
    Settings    string          `json:"settings"` // path edited
    Backup      string          `json:"backup"`   // pre-install copy
    Wrote       string          `json:"wrote"`    // sha256 of what install wrote
    Original    json.RawMessage `json:"originalStatusLine,omitempty"`
}
```

Uninstall removes exactly what install added: every handler whose command has the
marker (empty groups, empty event arrays and an empty `hooks` object that result
are dropped) and restores the stored statusLine (or deletes the key when there was
none). Two paths:

- The file is byte-identical to what install wrote (sha256 matches): the backup
  is restored verbatim, so the result is byte for byte the user's file.
- The user edited it since: surgical removal, re-serialised with sorted keys and
  two space indent plus a trailing newline; values inside are kept as written.
  This is the one documented formatting change. A backup of the edited file is
  made first.

Uninstall refuses a symlink, a non-regular file or invalid JSON the same way.

Status reports installed or not (by the markers, not by the state file), the
matched account, the backup, and whether a statusLine of the user's is chained.

### Hooks and statusline outside clalt run

`clalt hook` and `clalt statusline` decide in this order, after reading stdin:

1. CLALT_SESSION is a valid handle: the clalt run path, unchanged. This is the
   dedupe: a session with both the per-session --settings and the global hooks
   records through the registry path only, because the ext path never runs when
   the variable is set.
2. CLAUDE_CONFIG_DIR empty or the shared home: read `default-home.json` (one
   small read). Not matched: return at once, no output, no lock, no claude call.
   Any other CLAUDE_CONFIG_DIR is not the default home: return.
3. Matched: the session is keyed `ext-<claude session id>` (id from the input,
   charset [A-Za-z0-9-], at most 60 chars, else ignored). No registry record
   exists for it. The feed state is written with the matched account, limits from
   the statusline merge into the matched account, and the ledger gets one
   `default` line when the key is new.

feed.ValidKey accepts both shapes. Ext sessions have no process of their own to
look at, so liveness is the feed: shown while not ended and updated in the last
6 hours; pruneFeed drops one that is ended (after the usual 10 minutes) or idle
for 24 hours.

Dashboard: `RegistrySessions.Running` also lists the ext feed sessions (handle
`ext-...`, no wrapper pid, so no stop). The Profile of the matched account shows
the default home line.

## Decisions

- Marker as a trailing argument, not a shell comment: it survives sh and cmd and
  the hook already ignores extra arguments.
- Email match, not path match: claude does not say which clalt account a login
  belongs to, but the email is what the user sees and clalt recorded.
- No claude call outside doctor, feed and a background dashboard refresh.
- Ambiguity (same email twice) fails closed.
- The backup is a full copy, so even a broken surgical path loses nothing.

## Test plan (fake claude and temp HOME only, each test first)

defaulthome: match, no match, case difference, non-claude.ai, claude unusable,
ambiguous email. usage: default lines for unattributed only, already attributed
untouched, before the cutover untouched. feed install: merge with existing hooks
and statusLine, backup, user entries intact, uninstall byte for byte (untouched)
and semantic (edited), symlink and invalid JSON refused, idempotent. cli: hook
no-op and no stdout outside matched homes, ext record when matched, no double
record with CLALT_SESSION, dashboard lists the ext session, list and doctor lines.

## Before release (manual, real claude)

`clalt feed install --yes`, run `claude` in ~/.claude, confirm an ext session
shows in the dashboard and limits appear; `clalt feed uninstall --yes` and diff
settings.json against the backup.

## Review changes

- Cache gets `matchedSince`, reset when state or account changes. DefaultLines
  claims only sessions whose first record is at or after max(thinSince,
  matchedSince). A refresh that changes the match first flushes the sessions the
  old match earned (started before the change). A new ext session with a cache
  older than 5 minutes makes the hook refresh within its budget, else it is not
  recorded; the statusline never creates a session under a stale cache.
- Apply re-reads settings.json and aborts when it differs from what Prepare read;
  a failed write restores the previous state file.
- Commands: `[ -x BIN ] || exit 0; exec BIN hook <Event> clalt-feed`; statusLine
  falls back to the user's own command inline (`if [ -x BIN ]; then exec ...;
  else <user command> fi # clalt-feed`). State records `bin`; status and doctor
  warn when it is gone. A versioned or temp binary path is replaced by the link
  on PATH when it is the same file.
- Windows: hooks run in Git Bash, or PowerShell without it (docs, hooks page,
  `shell` field); no single command works in both, so feed install is refused.
