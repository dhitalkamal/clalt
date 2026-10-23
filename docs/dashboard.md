# Usage dashboard

`clalt dashboard` opens a full-screen interactive dashboard; `clalt usage`
prints the same data as static text (and the dashboard falls back to one static
frame when stdout is not a terminal). Both build stats from Claude Code's local
transcripts (`~/.claude/projects/**/*.jsonl`). `clalt budget` reads them too.
They are the transcripts of the shared claude home, also inside a `clalt run`
session, where `CLAUDE_CONFIG_DIR` points at an account folder. When they cannot
be read, `usage` and `budget` exit 1 and say why instead of reporting zero, and
the dashboard says so in its header.

The dashboard has an accounts panel (an estimate of each account's 5h use, with
a `!` on one that is signed out or has to sign in again within three days), a
sessions panel with the live `clalt run` sessions, and a detail pane for the
selected account: 5h and 7d windows, today's cost, trend, models, hours,
projects, email, budget and the sign-in. It shows the sign-in recorded in
`state.json`; it does not ask claude.

The percent in the accounts panel is an estimate, which is why it has a tilde
(`~17%`). It is the tokens that the transcripts hold for the account in the last
five hours, over a fixed 2,000,000. It is not a share of the plan's limit, which
nothing local knows, and it leaves out usage that is not attributed to an
account: that shows only in `clalt usage`. The bars in the detail pane use the
same reference. The real limits come with the statusline feed of the next step.

When something cannot be read (the transcripts, the folder of the sessions, or a
reload after an action), the header says so, and a reload that failed says so
instead of showing the old numbers as current. A `run` or `login` that was
refused prints why and waits for enter before the dashboard comes back.

Controls: `tab` and `shift+tab` move between panels, `up`/`down` (or `j`/`k`)
select, `enter` or `r` run the selected account, `n` sign an account in, `e`
set its budget, `R` rename it, `l` sign it out, `d` remove it, `/` filter,
`:` command palette, `?` toggle the keybinding overlay, `q` quit. In the
sessions panel `k` kills the selected session. Running and signing in happen
after the dashboard closes.

Token accounting: the headline "tokens" figure is input + output only. Cache
reads are excluded from that number (with a high cache-hit rate they are ~30x
real input/output and would make totals meaningless) and are shown separately as
the composition breakdown and cache-hit rate. Cost still includes cache.

Usage is split per account by an old switch log that nothing writes to any more.
From the first sign-in made with this version of clalt on, usage is not split
per account yet: `clalt usage` shows it, and anything else it cannot place,
under `unattributed`, and it counts against no account's budget. `state.json`
keeps the time of that first sign-in as `thinSince`.

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

The dashboard's detail pane shows the daily cap, and `e` sets both caps.

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
  `startedAt`. The command takes no arguments.
- The dashboard lists the same sessions in its sessions panel, where `k` kills
  the selected one.

## Preview of the new dashboard

`clalt dashboard --demo` opens the new focus-panes dashboard on made-up data, so
you can try it without touching any account. The current dashboard stays the
default and nothing here reads or changes your accounts. Keys: `j`/`k` move,
`tab` switches pane, `[` `]` or `1`-`5` switch view, `h`/`l` switch the history
tab, `/` filters, `a` toggles ascii mode, `?` lists every key, `q` quits and
`esc` never does. Enter, `s`, `e` and `x` only say what they would have done.
`NO_COLOR` turns the colors off and `CLALT_ASCII=1` starts in ascii mode.
Without a terminal it prints a single plain frame.
