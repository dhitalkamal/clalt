# Usage dashboard

`clalt dashboard` opens a full-screen dashboard of your accounts: who is signed
in, how much room each one has, what is running, and what it cost. `clalt usage`
prints the usage as static text. Without a terminal the dashboard prints one
plain frame instead. `clalt dashboard --demo` shows it on made-up data and
touches no account.

The left pane lists the accounts, the right pane shows the selected one in five
views, switched with `[` `]` or `1` to `5`:

- Overview: who is signed in and until when, the 5h and 7d limits, the budget,
  today and the week, and what is running.
- Sessions: the running claude sessions of the account.
- Limits: the 5h readings of the last seven days and the pace of both windows.
- History: usage per day, per project (enter opens a project) and per model,
  for the last 30 days. `h` and `l` switch the tab.
- Profile: the account folder, the claude version, whether the settings are
  shared with `~/.claude`, the login and the usage feed.

Where the numbers come from:

- Accounts, labels, budgets and when each signed in: `state.json`.
- Whether an account is signed in: `claude auth status`, asked for every account
  in the background (four at a time, and again after a minute at the earliest),
  never on every redraw. A call that fails keeps the last answer and puts a red
  `!` on the account. If claude is missing or too old, the header says so and the
  recorded state is shown.
- Usage: claude's transcripts, read in the background every half minute, with
  the attribution of `clalt usage`. The header says how much of the last 30
  days is not split per account yet (see below).
- Limits and running sessions come from the statusline feed and the session
  registry. Until they are connected an account shows `--` for its 5h usage,
  "No limit reading yet." in the overview, and the header says that running
  sessions are not shown yet.

What a failed read says: the header shows the first warning, a reload that
failed says so with the age of the numbers on screen, and every failure is
written with its full text to `~/.config/clalt/logs/dashboard.log` (one line
each, kept to about 512 KB, never any transcript text).

Keys:

```
up/down, k/j     move in the focused pane
tab              switch pane
[ ] or 1-5       switch view
h/l, left/right  switch the History tab
enter            run claude as the account (in History > Projects: open the project)
s                sign in
e                set the budget
r                rename the account (accounts pane)
d                sign out the account (accounts pane)
D                delete the account and its folder (accounts pane)
                 on a removed account: enter signs it back in, D deletes it
x                stop the selected session
/                filter the accounts
a                switch ascii mode
?                show the keys
esc              close help or the filter, go back from a project
q                quit
ctrl+c           quit from anywhere
```

`esc` never quits. `enter` and `s` run `clalt run` and `clalt login` for the
account, with the dashboard out of the way: the same checks, the same messages,
and when one refuses, the reason stays on screen until you press enter. When
claude ends, the dashboard comes back and reloads. `d` signs the account out and
keeps its history, like `clalt logout`; `D` deletes the account for good, like `clalt remove --delete`: a box names the
account, its email and the folder that goes, and the account label has to be
typed exactly before enter deletes it (esc cancels, a wrong label keeps the box
open). It refuses an account with a running session or a folder shared with
another account, and deletes nothing if the sign-out fails; the reason shows on
the footer line. Conversation history under `~/.claude` stays. `r` changes the
label only; `e` asks for
the daily and then the weekly cap, empty for none. `NO_COLOR` turns the colors
off and `CLALT_ASCII=1` starts in ascii mode.

Removed accounts (taken out of the list with `clalt remove`) show at the bottom
of the accounts pane under a dim `Removed` header, with their label and the
email on record. The section is left out when there are none. A removed account
counts toward no total, no room ranking, no limit and no session. The right pane
shows its label, email, folder and `removed`. Two keys work on it: `D` opens the
same type-the-label box and deletes it for good, and `enter` signs it back in,
like `clalt login <label>`, which brings it back into the list. Every other
action key says so on the footer line.

Token accounting: the headline "tokens" figure is input + output only. Cache
reads are excluded from that number (with a high cache-hit rate they are ~30x
real input/output and would make totals meaningless) and are shown separately as
the composition breakdown and cache-hit rate. Cost still includes cache.

Usage is split per account by an old switch log that nothing writes to any more.
From the first sign-in made with this version of clalt on, usage is not split
per account yet: `clalt usage` shows it, and anything else it cannot place,
under `unattributed`, and it counts against no account's budget. `state.json`
keeps the time of that first sign-in as `thinSince`. The dashboard shows what
it can place and says in its header how much it cannot.

## Budgets

`clalt budget` shows what each account spent today and over the last seven days
next to its caps, in dollars. A cap is set on one account at a time:

```
clalt budget                    every account, with its spend and its caps
clalt budget work --daily 20    daily cap of work: $20
clalt budget work --weekly 100  weekly cap of work: $100
clalt budget work --clear       remove both caps of work
```

- A cap is a number of dollars, 0 or more. Setting one cap leaves the other as
  it is, and `--daily 0` or `--weekly 0` removes that cap. `--clear` removes
  both.
- The name is a label, like everywhere else, and an account that does not exist
  or was removed is an error. Only one name is taken, and it is needed to set or
  clear a cap. `clalt budget work` with no cap shows the same table as
  `clalt budget`. After a cap is set, the table follows a line that says which
  account was updated (not with `--json`).
- A negative number, a value that is not a number (`nan`, `inf`, text), a flag
  that does not exist, a second name or a cap without a name is a mistake in the
  arguments: the command prints its usage, exits 2 and changes nothing.
- The spend is estimated from the local transcripts, per account, as in
  `clalt usage` (see "Token accounting" above). Usage that cannot be placed
  under an account counts against no cap.
- `--json` gives one object per account: `id`, `name` (the label), `dailyBudget`,
  `weeklyBudget`, `todayCost`, `weekCost` and `overDaily`, which is true when a
  daily cap is set and today's spend has reached it. Setting a cap with `--json`
  prints that table and nothing else.

The dashboard's Overview shows the caps next to what was spent, and `e` sets
both.

## Sessions

`clalt sessions` lists the `clalt run` sessions that are alive, oldest first:

```
ACCOUNT  PID    UPTIME  CWD
work     21501  12m     /home/k/clalt
```

Each `clalt run` writes a small record (the account, the process id, the working
folder and the start time) to `~/.config/clalt/sessions/` and removes it when
claude exits. A record whose process is gone, after a crash for example, is
dropped the next time the sessions are listed. Accounts run in parallel, so an
account can be there more than once.

- ACCOUNT is the label the account has now, so a rename after the session
  started shows. A session whose account no longer exists shows its id.
- Without a session it says so and shows how to start one.
- `--json` gives an array of `account` (the id), `label`, `pid`, `cwd` and
  `startedAt`, plus `handle`, `folder`, `wrapperPid`, `claudeSessionId` and
  `terminal` (`kind`, and the kitty, tmux or IDE details) from the registry, see
  [feed.md](feed.md). The command takes no arguments.
- The dashboard's Sessions view is fed by the session registry that comes with
  the statusline feed, and stops a session with `x`.
  `x` asks again before signalling anything: the record must be live and on that
  account, and the pid a claude that started after the session, as a child of its
  `clalt run`. Otherwise nothing is stopped and the dialog says why. claude then
  gets SIGTERM, and `clalt run` cleans up when it exits. Windows does not stop
  sessions from the dashboard yet.
- The dashboard has no reading history, so the Limits chart says "No readings
  yet". A window the feed lacks shows as "not reported", and a reading whose
  reset time has passed shows as reset, with the reading's age.
