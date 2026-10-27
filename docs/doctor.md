# Checking your setup

```
clalt doctor
```

One line per check, `ok`, `warn` or `fail`, and exit code 1 if any check
fails. It never changes anything: `clalt run` repairs links, and old files are
for you to delete.

`fail` is for what is really broken: claude missing or too old, an account id
that cannot be a folder name, two accounts that share one folder, a `projects`
folder in an account that has something in it (its history would not be
shared), a `state.json` that cannot be read, and a login of a different email
than the one recorded. What you fix by doing something, such as signing in or
mounting a volume, is a `warn`: not signed in, a folder that is missing, a
login that is due, links or servers out of sync, a shared history folder that is
not reachable, something claude said about an account, variables, settings,
leftovers, or a check that could not run. So doctor after a plain `logout`, or
right after upgrading from v0.1.0, exits 0 with warnings.

- claude is on PATH and at least 2.1.301 (older versions could sign a folder
  out when `claude auth status` exited in the middle of a token refresh).
- `state.json` can be read: an `ok` line says so, or says that there is no
  account list yet, before the first sign-in. When it cannot be read, that is one
  failed check, and the checks that need no accounts still run.
- For each account: the folder is there and is not shared with another account,
  its links and MCP servers are in sync, it is signed in, signed in as the
  recorded email, and its login is not due within three days. Links are judged
  the way `run` is going to find them: a shared `projects` folder that is
  missing, or an empty `projects` folder in the account, is a warning, since
  `run` makes the one and replaces the other with the link.
- For each account that has rules (see [move.md](move.md)): a `rules` line. It
  is `warn` when a rule is for a folder that is gone or is not an absolute path,
  since such a rule matches nothing, and says to remove it with `clalt rules
  <name> --remove-folder <folder>`. An account with no rules gets no line.
- For each account you removed (they are not in the list any more): when its
  folder is there, claude is asked about it. A folder that still holds a login
  warns with `clalt logout <name>`, which signs it out and leaves the account
  removed, and so does a folder claude could not be asked about. A removed
  account with nothing to say gets no line.
- What claude wrote to stderr when it was asked who is signed in to an account,
  such as a `.claude.json` it found corrupt and set aside. It is shown as a
  warning for that account.
- Auth variables set in the current shell, and an `apiKeyHelper` or auth
  variables in the shared `settings.json` and `settings.local.json` (names only,
  never values; the list is in [auth.md](auth.md)).
- The session feed: sessions that started and never reported (their hooks are
  off), records and feed files of sessions that are gone, and the hook errors of
  the last 24 hours (see [feed.md](feed.md)). All of these are warnings.
- The default home: which account plain `claude` (`~/.claude`) is signed in as,
  or that it stays outside clalt and why, and whether the live feed is installed
  for it. It reads the cache of that answer, so it never asks claude and says
  when it was checked; `clalt feed status` refreshes it.
- Leftovers of the old design: the token vault, the old profile folders, the
  old keepalive job and the marker files it left in `~/.config/clalt/`, and on
  macOS a `~/.claude/.credentials.json`. What to delete is in
  [upgrading.md](upgrading.md).
  - Old profile folders are a warning for as long as one is in
    `~/.config/clalt/profiles/`. v0.1.0 staged a plaintext login in some of them
    (a `.credentials.json`), so doctor names each one it finds and says to
    delete those files. It only checks that the file exists and never opens it.
    It also says whether the history in the folders is copied, and points at
    `clalt migrate` when it is not. When nothing is left to copy and only
    files that differ from the shared copy remain, it says all history is
    already in the shared folder and names those files. That part needs the shared home, so inside
    a `clalt run` session or with `HOME` unset it says it was not checked.
  - `~/.claude/.credentials.json` may be a leftover of the old switch flow, but
    claude writes it too when it cannot save its login in the keychain, and then
    it is your real login. Doctor asks claude itself (`claude auth status` for
    the default claude home) and never opens the file. Signed in with a
    claude.ai login: ok, the file is the live login and must stay. Not signed
    in: a warning that the file looks like a leftover and can go. If claude
    cannot answer, or the login is not a claude.ai one, the warning says to run
    `claude auth status` from a normal shell first.
