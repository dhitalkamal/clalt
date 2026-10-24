# Commands

The short list is in the [readme](../README.md). This is what each command does.
Accounts are named by their label on every command line, ignoring case, and
most commands take `--json`. The label `unattributed`, in any case, is reserved:
usage reports use it for usage that no account can claim, so `login` and
`rename` refuse it.

Exit codes: 0 when it worked, 2 for a mistake in the arguments (it prints the
usage) and for an unknown or retired command, 1 for anything else that went
wrong, and for `run` the exit code of claude. A report that cannot be written,
on a full disk for example, is a failure too: exit 1 and `the output could not
be written` with the reason. When the reader of a pipe goes away before clalt
has written everything, clalt ends by SIGPIPE without a message, as most tools
do. `doctor` exits 1 when a check fails.

## Signing in

`clalt login <name>` runs `claude auth login` in the account's folder and then
asks claude who signed in. It records the email, plan and organization with the
time. If your browser is already signed in to another claude.ai account, use a
private window for the sign-in: clalt prints that tip on stderr before it
starts.

clalt keeps the sign-ins apart:

- Signing an account in again passes the recorded email to claude, so the
  browser form is pre-filled.
- If the email that signed in already belongs to another account, or differs
  from the one recorded for this account, clalt signs the folder out again and
  refuses. It names the email that signed in and the account or email it clashes
  with, and asks you to try again in a private window.
- A new name makes the account when you sign in. If that first sign-in is
  refused or fails and nothing is left signed in, the account is hidden again
  instead of staying in `clalt list`, and signing in under the same name reuses
  it. When claude may have left a login in the folder (it could not be asked, or
  an `apiKeyHelper` overrides it) the account stays in the list, so that you can
  sign it out.
- `logout` keeps the recorded email, so the next sign-in is still checked.
  `remove` forgets it, so a later `clalt login <name>` starts a fresh identity.
- When the account's history does not reach the shared history, `login` prints
  one warning line after it has linked the account and goes on with the
  sign-in: `run` will refuse the account until it does (see
  [sharing.md](sharing.md)).

A sign-in lasts 30 days (claude caps its refresh grants there). `list` and
`whoami` show the date to sign in again by, and `doctor` warns three days ahead.

## Running accounts

`clalt run <name>` does this before it starts claude:

1. links the account folder to the shared claude home and mirrors your
   user-scope MCP servers into the account (it says on stderr what it changed,
   see [sharing.md](sharing.md));
2. removes the variables that would override the folder's login and says on
   stderr which of them were set, by name and never by value (see
   [auth.md](auth.md));
3. asks claude who is signed in, and refuses unless it is a claude.ai login in
   this folder, as the email that was recorded. Each refusal prints the command
   that fixes it, usually `clalt login <name>`.

When something outside the account overrides its login (an `apiKeyHelper` or a
provider setting in the shared settings), it says so and points at
`clalt doctor`, since signing in again would not change it.

Before it starts claude, `run` also checks that the account's history is the
shared history, and refuses if it is not: see "The history check" in
[sharing.md](sharing.md).

While the session runs, clalt stays until claude is gone. At a terminal a
Ctrl-C reaches both of them, and what it means is claude's to decide, so clalt
only waits: passing it on as well would give claude a second one, which can turn
an interrupt into an exit. A SIGTERM or SIGHUP sent to clalt alone (`kill`, an
IDE that stops it, a closed terminal) is passed on to claude, so that stopping
clalt does not leave claude running. So is a SIGINT when no terminal sends a
Ctrl-C to clalt (a headless `-p` run, a script, a supervisor): then a `kill -INT`
at clalt would be lost. "At a terminal" means that clalt's input is a terminal,
or that clalt is in the foreground process group of its controlling terminal,
as it is for `echo hi | clalt run work -p` typed at a shell: that Ctrl-C
reaches claude by itself. On Windows only the input decides. clalt exits with
the code claude ends with. A signal that was ignored when clalt started, under `nohup` or in a job
that a script puts in the background, stays ignored for clalt and for claude, as
it would for claude run by hand.

Each session gets a handle in `CLALT_SESSION`, a record in
`~/.config/clalt/sessions/`, and hooks and a status line that report what it is
doing, passed to claude with `--settings` and never written to your settings (see
[feed.md](feed.md)). `clalt hook` and `clalt statusline` are the commands claude
runs for that; they are not meant to be run by hand.

Everything clalt says while it gets the session ready is on stderr, so stdout is
claude's alone: `clalt run work -- -p "..." | jq` gets claude's answer and
nothing else. Everything after the account name goes to claude, `--json`
included.

## Plain claude

```
clalt feed install [--yes]     add the hooks and status line to ~/.claude/settings.json
clalt feed uninstall [--yes]   remove exactly those
clalt feed status [--json]     installed or not, which account plain claude is
clalt feed forget-default      undo the usage attributed to plain claude
clalt feed remember-default    attribute it again
```

These give plain `claude` the live feed, when it is signed in as the same email
as one of your accounts. The details, the backup and what uninstall restores are
in [feed.md](feed.md).

## Managing accounts

```
clalt list                 label, email, plan, signed in, login due, last used
clalt whoami               the account of this shell, with its live state
clalt logout <name>        claude auth logout in the folder; history stays
clalt remove <name>        sign out unless claude says nobody is signed in, then
                           hide; history stays
clalt remove <name> --delete   sign out, then delete the folder and the record
clalt rename <old> <new>   label only, the folder never moves
```

`list` asks claude about every account, a few at a time, and shows `unknown`
with the reason when it cannot get an answer. `list --json` gives `id`,
`label`, `email`, `org`, `plan`, `signedIn` (true, false or null), `reason`,
`signedInAt`, `loginDueAt`, `lastUsedAt`, `folder` and `defaultHome` (true for the
account that plain `claude` in `~/.claude` is also signed in as, from the cache
of the default home; the table says it in a line under it).

`whoami` looks at `CLAUDE_CONFIG_DIR`: inside an account folder (which is what
a `clalt run` session looks like) it shows that account; inside an old v0.1.0
profile it says so; anywhere else it says this shell is not in a clalt account
and plain `claude` uses the normal login.

`logout` works on a removed account too: it signs the folder out and leaves the
account removed. `remove` asks first unless it gets `--force`. Any answer but
yes keeps the account, and when nobody can answer (the input is closed) it exits
1 and says to pass `--force`.

If signing out fails, `logout` and `remove` say so and change nothing, so no
login is left behind in a hidden folder. When the sign-out worked but
`state.json` could not be written, they say that the folder is signed out and
that the record was not saved, and that running the command again tries again. After a sign-out claude is asked for the
status to confirm it worked. When the call was cut short, or the check could not
be made, the sign-out state is unknown: they say so, keep clalt's record and the
account as they were, and `clalt list` shows where it stands. `remove` never
deletes the folder: the account is hidden, and `clalt login <name>` brings it
back in the same folder.

`remove <name> --delete` goes further and deletes the account for good, which
works on a removed account too. It signs the folder out first and stops, deleting
nothing, if the sign-out fails or claude cannot say whether the folder is signed
in. It refuses a folder shared with another account, and a folder that is not a
real folder directly under clalt's accounts folder. It deletes only that folder:
the links inside it into `~/.claude` are removed, never followed, so shared
history and settings stay. It names the account, its email and the folder, and
asks first unless it gets `--force`; with no answer it exits 1. The record is
dropped from `state.json`. Past usage still shows the account by its id.

## Rules, picking and moving

`clalt rules <name>` shows and edits the folders an account may serve and
whether auto-switch may use it. `clalt run` with no name picks the allowed
account with the most room. `clalt move <session> <account>` moves a running
session to another account at its next pause. All three are in
[move.md](move.md), with the terms that apply to auto-switch.

## Resume, launch, jump and alerts

`clalt resume`, `clalt launch`, `clalt jump` and `clalt notify` are described in
[extras.md](extras.md).

## Usage, budgets and sessions

`clalt usage [name]`, `clalt budget [name]`, `clalt sessions` and
`clalt dashboard` are described in [dashboard.md](dashboard.md), with
[budgets](dashboard.md#budgets) and [sessions](dashboard.md#sessions) each in a
section of their own. They take and show labels like the other commands.

## Office

`clalt office [--port N] [--no-open] [--edit] [--template <id>]` opens a live 3D office of your
agents in the browser, on this machine only and watch only (`--edit` adds build
mode). `clalt office layout list|use|export|import|reset` manages the floor
plans. Both are described in [office.md](office.md).
