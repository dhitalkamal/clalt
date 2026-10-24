# clalt v2 step 3b: the dashboard on real data

Task brief 2026-10-23, implementing the dashboard part of step 3 in
docs/plans/clalt-v2.md. Branch feat/dashboard-real, off develop at 7699db9. The design
(docs/plans/dashboard-v2.md and its frames) is kept as approved.

## Scope

1. internal/dashboard/live: real Source and Actions for the ports of internal/dashboard.
2. `clalt dashboard` opens the new dashboard on real data. `--demo` stays. The old
   dashboard (internal/tui and everything only it used) goes, and go.mod is tidied.
3. Known issues: the 5 second wait at the start of every command, the test that hangs on
   a terminal, design/tui-mock.html, and docs/dashboard.md for the new dashboard.

Not here: the session registry, the feed, the hooks and cmd_run.go (step 3a), moves (step 4), per-session usage attribution, an incremental
transcript cache. Where one of them is needed there is an interface with a default that
returns nothing. A wiring pass after step 3a merges connects them.

## What each port method reads or does

Source.Load builds one Snapshot. Nothing in it is read per frame.

- Accounts, labels, order: state.json through account.Store. Visible accounts only,
  sorted by label (ignoring case) and then id, like `clalt list`.
- Email, org, plan, signed-in time, last used, budgets: the state.json record. The plan
  reads "<type> plan".
- SignedIn, Problem, and the live email, org and plan: `claude auth status --json`,
  asked through Host.CheckSignIn, which is cli's own rule from `clalt list` and
  `clalt run` (claudecli.Status, then the recorded email). See decision 3.
- Login: the recorded sign-in time, due 30 days after it.
- History (days, projects, models) and Account.LastIn: the transcripts, through
  Host.Usage, which is the attribution `clalt usage` uses. See decision 4.
- Limits, Profile.FeedInstalled: Feed. Sessions: Sessions. Gateways: Gateways.
- Profile: the account folder (config.AccountDirFor), the claude version (Host, cached
  5 minutes), SharedSettings (Host.LinksInSync: the links of the folder are in sync with
  the shared claude home).
- Warnings: claude missing or too old, usage that could not be read, usage not split per
  account yet (with its cost), a feed, sessions or gateways read that failed, and
  "running sessions are not shown yet" while Sessions is the default.

Actions, all addressed by account id:

- Run and SignIn: a Launch that runs the code path of `clalt run <label>` and
  `clalt login <label>` in this process with the terminal released. See decision 2.
- Rename: Store.Relabel(id, label). SetBudget: Store.SetBudget(id, daily, weekly), after
  checking the amounts are finite and not negative (the store reads a negative as
  "leave it").
- SignOut: cli's signOut (claude auth logout, check, then the record), through Host.
  Afterwards the cached sign-in of the account is replaced.
- StopSession: Sessions.Stop. The default says it is not available yet, as a Notice.

An id is checked with config.CheckAccountID before it becomes a path. A label is never
put in a path. A label that starts with "-" (old state files were not checked) is
refused for run and login, since it would be read as a flag.

## Interfaces defined here (package live)

My own minimal types, so nothing imports step 3a or step 6 while they are unmerged.

    type Host interface {                         // implemented by cli
        CheckSignIn(acc account.Account) (SignIn, error)
        ClaudeVersion() (string, error)           // version, and why it cannot be used
        LinksInSync(folder string) bool
        Usage() (map[string][]usage.Record, error) // by account id, usage.Unattributed
        SignOut(acc account.Account) error
        Run(acc account.Account) dashboard.Launch
        Login(acc account.Account) dashboard.Launch
    }
    type Feed interface {
        Limits(ctx, accountID string) (Limits, bool, error)   // false: nothing recorded
        Readings(ctx, accountID string, since time.Time) ([]Reading, error)
        Installed(accountID string) bool
    }
    // Limits: Five and Seven (Percent, ResetsAt) and ObservedAt.
    // Reading: At and FivePercent, the history behind the Limits chart.
    type Sessions interface {
        Running(ctx) ([]Session, error)
        Stop(ctx, accountID, handle string) error
    }
    // Session: Handle, AccountID, ClaudeSessionID, State (starting, busy, idle,
    // needs-you, ended), Reason, Model, Cwd, StartedAt, UpdatedAt.
    type Gateways interface{ Running(ctx) ([]Gateway, error) } // Gateway: AccountID, Addr

These follow the shared contracts of step 3a (registry record, feed fields). Defaults
are NoFeed, NoSessions and NoGateways: no limits, no sessions, no gateways. Session
states map to the dashboard's: busy is working, needs-you is waiting, starting and
idle are idle, ended is not listed.

## Decisions

1. live does not import cli. cli imports live to wire it, so the reverse is a cycle. The
   operations stay where they are and reach live through Host, implemented in
   internal/cli/cmd_dashboard_live.go. Moving them into a shared package would touch
   cmd_run.go, cmd_login.go, cmd_logout.go and core_calls.go, which step 3a and the
   other builders are editing.

2. Run and SignIn become launches. The port changes from *exec.Cmd to Launch, an
   interface with the four methods of tea.ExecCommand; dashboard.Command wraps an
   *exec.Cmd. Why: `clalt run` has to stay the only way into claude, because the wrapper
   registers the session (step 3a) and moves it later (step 4), and a claude command
   built by the dashboard would skip that. A command that starts the clalt binary again
   cannot keep a refusal readable: the screen comes back at once and wipes it, which the
   old dashboard avoided by waiting for enter (CHANGELOG). In process, the launch calls
   cmdRun or cmdLogin, prints a failure and waits for enter, then returns, so the
   dashboard reloads. claude's own exit status is passed on and reads "claude exited with
   status N" as before. A launch that returns in under 50 ms is held for the rest,
   since bubbletea v2.1.0 can stall when an exec returns at once.

3. Sign-in checks are cached, in the background, and never cut short. Per account: 60 s
   cache, at most 4 calls at once, refreshed ahead of a load that needs it. A load waits
   at most 3 seconds for an account with no answer yet (first load, or reset after a
   launch or sign-out); after that it shows the record and the answer arrives with a
   later load. Stale answers are used while a new one is fetched. The calls do not use
   the load context: a 30 second load timeout would interrupt `claude auth status` in
   the middle of a token refresh (thin-profiles.md, Calling claude). A failed check keeps
   the last answer and sets Problem. This replaces the 5 second timeout of
   dashboard-v2.md. A claude that is missing or older than the minimum is one warning
   and the checks are skipped.

4. Usage is the attribution `clalt usage` has, and it is read in the background. Host.Usage
   in cli is cli's recordsByAccount, so the dashboard follows whatever attribution cli
   gets later. The scan is single flight, kept 30 seconds, and the first load waits for it
   at most 3 seconds (then shows no history and a warning). Only the built history is
   kept, not the records. Known limit: usage since the first folder sign-in has no account
   (thin-profiles.md, state.json), so those accounts show little history. The header says
   how much of the last 30 days is not split, instead of showing zero as fact.
   Project names come from the working folder of the records (home as ~, shortened to the
   last element, longer only where two names collide); model ids are shown as "Opus 5.5".

5. Limits come from Feed and are mapped, not trusted. A window whose reset time has
   passed counts as 0% with no reset time (the reading is old, ReadAt says how old). The
   forecast uses the reading: pace = percent / elapsed share of the window, Projected is
   the percent at the reset, RunsOutAt is when it reaches 100, set only before the reset.
   No forecast before 5% of the window has passed. The chart buckets readings into 3 hour
   slots over 7 days (the highest percent of each slot).

6. Views say what is missing. With no limit reading the list shows "--" and the overview
   shows "No limit reading yet." and no bars (they would read as 0%); "most room left"
   ignores such accounts. A session shows lines changed only when it has some, and the
   profile view leaves out " in <folder>" when the folder is unknown.
   dashboard.Render now cleans the snapshot like the program does (watch item of PR 1).

7. Interactive means the app's own streams. cmdDashboard asked os.Stdout, so `go test`
   with stdout on a terminal started the program and hung (TestDashboardDemoStill
   RunsAndExitsZero). It now needs a.In and a.Out to be terminals. The test that pins it
   points os.Stdout at a terminal device and a.Out at a buffer.

8. The 5 second wait. Confirmed before any change: on a terminal that does not answer
   a colour query, `clalt version` took 5.01 s (0.005 s on a pipe), and a stack dump
   showed it in the package init of bubbletea v1.3.10 (tea_init.go calls
   lipgloss.HasDarkBackground, termenv asks the terminal and waits OSCTimeout, 5 s).
   Only internal/tui imported v1, so deleting it removes the init. A test pins that go.mod
   asks for no Charm v1 library, and the timing is measured again after the deletion.

9. Failures are logged to ~/.config/clalt/logs/dashboard.log (Options.Logger, mode 0600,
   one line each, cleaned, rotated at 512 KB). The screen shows the first line only.

10. No flag served only the old dashboard. `--live` is shared with usage, and its
    retired-flag message (pinned by tests, in upgrading.md) stays.

## Verified

- The 5 second wait: `clalt version` on a terminal that never answers took 5.01 s
  before and 0.41 s after the deletion (pty harness, temp HOME).
- The hang: the cli test binary on a pty ran the demo dashboard for the 8 s limit with
  the old check and passed in 0.48 s with the new one.
- End to end on a pty with the built binary, a temp HOME and the fake claude: the list
  with live sign-in state, enter (session started in the account folder, dashboard
  back and quit with 0), a refusal that stays until enter, `s` (sign in recorded) and
  `d` then `y` (signed out).

## Tests

Test first for every change. live tests use fakes for Host, Feed, Sessions and Gateways
and a real account.Store in a temp dir. cli tests use the fake claude of
fakeclaude_test.go and a temp HOME. Nothing runs the real claude or reads real accounts.
The end-to-end check drives the built binary on a pty with a temp HOME and the fake
claude.

## PR

One PR, feat/dashboard-real into develop, commits in this order: this plan; the Launch
port; the views; live (cache, accounts and sign-in, history, limits and sessions,
actions); the cli wiring and the removal of internal/tui; the known issues; docs.

## Open questions

- Is the Launch port change acceptable? The alternative is a command that starts
  `clalt run <label>` again, which loses the reason of a refusal.
- Does the feed keep a reading history? The Limits chart needs one, and the step 3a
  contract lists only the current limits. Without it the chart says "No readings yet."
- Session branch, effort, context size and lines changed are not in the contract. The
  sessions view omits them until a source exists (git read, or feed fields).
- Per-session attribution of usage: in step 3a or later? Until then new usage shows as
  not split.
- The rename entry accepts spaces and non-ASCII that account.ValidateName refuses, and
  the refusal comes back as a red message after the entry closes. A validation hook in
  Options would keep the entry open.
- Wiring pass: Feed, Sessions (list and stop, with the pid check of the threat model)
  and Gateways adapters, and the Snapshot field for gateways (step 6).
