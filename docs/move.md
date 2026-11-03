# Account rules, picking and moving sessions

An account can be given rules: the folders it may serve, and whether
auto-switch may use it. The rules decide the choices clalt makes for you. They
never stop you from running an account by name.

## Account rules

```
clalt rules <name>                       show the rules of an account
clalt rules <name> --add-folder PATH     add a folder (repeat for more)
clalt rules <name> --remove-folder PATH  remove one
clalt rules <name> --auto-switch         let auto-switch use this account
clalt rules <name> --no-auto-switch      stop it
```

`--json` gives `id`, `label`, `folders`, `anyFolder`, `autoSwitch` and
`problems` (rules that cannot match, with the reason).

- A folder rule matches that folder and everything under it. Links are
  followed on both sides, and on a file system that ignores case (macOS and
  Windows by default) so does the match.
- An account with no folder rules may serve any folder. Removing the last
  folder puts it back to that, and clalt says so.
- A relative path is taken against the folder you are in when you type it, and
  stored as an absolute one. A `~` that the shell did not expand is refused.
- Removing takes the folder however it is spelled, a link to it included, and
  fails when there is no such rule, so a mistyped folder does not look as if it
  worked. A folder that does not exist can be added, with a warning.
- `clalt doctor` has a `rules` line for each account that has rules, and warns
  about a rule for a folder that is gone or that is not an absolute path,
  because such a rule matches nothing.

## Picking an account

`clalt run` with no name (or `clalt run -- <claude args>`) picks the account
for you and says on stderr which one and why. A flag before the name is still
an error: everything for claude goes after `--`.

- It looks at the accounts that were not removed, have a sign-in recorded, and
  whose folder rules allow the folder you are in. The auto-switch flag does not
  matter here.
- The one with the most room comes first. Room is 100 minus the higher of the
  5-hour and 7-day used percentage that the feed last saw. A window whose reset
  time has passed counts as empty.
- Accounts with no limit data from the last 24 hours rank after those with
  data, and ties go by label. With no data at all it is label order.
- The one that wins is asked of claude whether it is signed in. If it is not, it
  is skipped (with a line saying so) and the next one is tried.
- When nothing can be picked, the error lists each account and why it was left
  out. Run one by name to override: `clalt run <name>` ignores the rules.

## Moving a session

```
clalt move <session> <account>
```

The session is named by its handle (16 characters), the pid of its `clalt run`,
or the start of its claude session id (at least 4 characters). The account is
named by its label. This asks the session to move; it does not wait. The move
happens at the next pause, which is when the session is idle: claude has
answered and nothing is running, for at least a second and a half.

`move` refuses at once when the session is already on that account, the account
was removed or is not signed in, its folder rules leave out the folder the
session is in, claude has not reported the session id yet, or the feed has no
state for the session (its pause could not be seen).

At the pause the wrapper of the session (the `clalt run` process that started
claude):

1. gets the target ready first, while claude still runs: rules, links to the
   shared home, the variables that would override the login, claude's word that
   it is signed in. If that fails, the session stays as it is and one line says
   why when claude ends.
2. ends claude with SIGTERM and gives it 15 seconds, then kills it. claude saves
   as it ends, and every reply is already in the transcript at a pause.
3. puts the terminal back, prints one line (`clalt: moved session 3fa9c1d2 from
   work to personal: requested with clalt move`) and starts `claude --resume
   <session id>` in the same terminal and folder, under the other account.
4. if that start fails, resumes under the account it was on and says so.

The resume carries the flags of the first start that a resume does not restore
(`--mcp-config`, `--settings`, `--plugin-dir`, `--fallback-model`, `--add-dir`,
and the flags that bypass permissions). It does not repeat the model, the
permission mode, the agent or the first prompt: the resume restores what you
changed in the session.

What a move costs:

- An unsent draft in the prompt box is lost.
- Background commands and subagents of the session end with claude.
- The new account reads the whole conversation again, with no shared prompt
  cache, which uses part of its limit.
- A prompt typed in the instant between the pause and the signal is left as an
  interrupted turn, and claude says so when it resumes.
- Windows cannot send the signal, so `move` refuses there.

Everything the wrapper needs from the registry and the feed comes from the
session registry and the session feed. `clalt run` starts claude through the
wrapper (everywhere but Windows), so a session it runs can be moved. A session
whose claude has not reported its session id yet, or whose hooks are off (a
`--settings` of your own), has no pause to act at, and a request waits.

## Auto-switch

Off unless you turn it on for an account with `clalt rules <name>
--auto-switch`. It moves a session with no request, at a pause, when all of
this holds:

- auto-switch is on for the account the session is on, and for the target;
- the session's account has used 90 percent of its 5-hour window, or claude
  reported that the limit was reached;
- the target is allowed in the folder, signed in, has limit data from the last
  24 hours, and has at least 20 points more room than the account it leaves
  (all of it, when the limit was reported);
- the session has not moved in the last 30 minutes, and the target did not fail
  to start in the last 30 minutes.

Nothing qualifying means nothing happens, and nothing is printed. A move prints
the same single line as a requested one, with the reason: `5-hour window at 93%
(switches at 90%)` or `claude reported the limit`. A request from `clalt move`
wins over an automatic move. The threshold, margin and cooldown are not
settings yet.

## Terms

Auto-switch is off by default and per account. It is meant for accounts you
are entitled to use for the work at hand, for example your own personal account
and your own work account. Each account's terms of service apply to how you use
it, and moving a session between accounts does not change that. Turning
auto-switch on prints a line that says so.

Do not use moves or auto-switch to combine several subscriptions into more
usage than your plans give you. Anthropic's
[Usage Policy](https://www.anthropic.com/legal/aup) does not allow bypassing
restrictions in its products, and plan limits assume ordinary, individual use
([Claude Code legal and compliance](https://code.claude.com/docs/en/legal-and-compliance)).
See also the terms section in the [readme](../README.md).
