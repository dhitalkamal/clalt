# Resume, launch, jump and alerts

Four commands that work on sessions rather than accounts. They use only what is
already on the machine: clalt never turns on kitty remote control and never
changes kitty or tmux configuration.

## Resume

```
clalt resume [words] [--limit n] [--json]
clalt resume <id> [--account <name>]
```

`clalt resume` lists the conversations in the shared projects folder
(`~/.claude/projects`), which holds those of every account, newest first. Each
row shows the first 8 characters of the id, when it was last used, the model,
the size, the folder it ran in and a title. The title is the name given to the
session, else the title claude generated, else an older summary record, else the
first prompt, cut short. Twenty rows are shown; `--limit n` changes that and
`--limit 0` shows all of them. Words filter the list: every word has to appear,
ignoring case, in the id, title, folder or model.

`--json` prints an array with `id`, `title`, `cwd`, `model`, `lastActive`
(RFC 3339, UTC), `size` in bytes, `project` (the folder name claude uses) and
`entrypoint` (`cli`, or `sdk-cli` for conversations that were not started from
an interactive terminal, such as headless runs).

An argument of at least 8 characters made only of 0-9, a-f and dashes is taken as
a conversation id and runs it. Anything else is a filter. An id that matches
nothing is an error, and one that matches several lists them and stops.

Running a conversation goes through the same path as
`clalt run <name> -- --resume <id>`, started from the folder the conversation
ran in, so claude finds it in its own project. Every check `run` makes still
applies. The account is `--account <name>`, else the account that last ran the
conversation when clalt knows it, else the command stops and asks for
`--account`.

It stops, and runs nothing, in two cases:

- The folder the conversation ran in no longer exists. The message says so and
  names `clalt run <name> -- --resume <id>`, which resumes it from the folder you
  are in.
- The conversation is open in a running session. Two terminals on one
  conversation write into one transcript. The message names `clalt jump` and, to
  do it anyway, the same `clalt run` command.

Listing makes no state files and reads only the transcripts it needs: files are
taken newest first and read until enough match. A filter that matches little, or
`--limit 0`, reads every transcript once (about two seconds for 600
conversations).

## Launch

```
clalt launch <name> [--cwd dir] [-- claude args]
```

Opens `clalt run <name>` in a new tab or window of the terminal that is already
there:

- inside tmux (`TMUX` is set): a new tmux window, `tmux new-window`;
- else with kitty remote control on a socket (`KITTY_LISTEN_ON` is set): a new
  tab, `kitten @ --to <socket> launch --type=tab`;
- else nothing is opened and the command to run is printed.

tmux wins when both are there, since it is the innermost one. `--cwd` is the
folder the tab opens in, the current one by default, and has to exist.

Before a tab opens, launch checks that the account exists, was not removed and is
signed in as recorded, because a tab whose command fails closes with it and the
error would be lost. The tab runs `env ... <clalt> run <name>`: kitty and tmux
start it with their own environment, so `PATH`, `XDG_CONFIG_HOME`,
`CLAUDE_CONFIG_DIR` and `CLALT_CLAUDE_BIN` are passed on from your shell when
they are set, and nothing else.

To use launch from kitty, remote control has to be on already, for example
`allow_remote_control socket-only` and `listen_on unix:/tmp/kitty` in
`kitty.conf`. clalt does not set this up.

## Jump

```
clalt jump <session>
```

Brings the terminal of a running session to the front. `<session>` is a handle,
or the start of one, or the start of the claude session id. Several matches are
listed and nothing is focused.

- kitty: `kitten @ --to <socket> focus-window --match id:<window>`, which also
  switches to the kitty OS window that holds it.
- tmux: selects the window and the pane, then switches the tmux client to them.
  Outside tmux there is no client to switch, so that last step may only be
  reported: the window and pane are still selected for whoever attaches next.
- an IDE or a plain terminal: clalt cannot raise it, so it prints where the
  session runs and exits 0.

A kitty id that is not a positive number, an address that does not start with
`unix:` or `tcp:`, and a tmux pane that is not like `%5` are refused without
running anything.

## Needs-you alerts

```
clalt notify [on|off|status|test]
clalt notify idle on|off
```

When a session waits for you, clalt shows a desktop notification: the account,
what claude is waiting for and the last element of the folder. Nothing from the
conversation and no command is included. At most one is shown per session per
wait; the next one comes after the session has gone back to work. The alerts are
on by default.

- macOS: `osascript` with the text passed as arguments of the script, never as
  part of it. macOS attributes the notification to Script Editor, from what is
  known of it, so if none appear, check that notifications are allowed for it in
  System Settings.
- Linux: `notify-send`, when it is installed.
- elsewhere: nothing.

`clalt notify off` and `on` change the setting, kept in
`~/.config/clalt/notify/settings.json`; a settings file that cannot be read
counts as on, and `status` says so. `status` also says how a notification would
be shown on this machine. `test` shows one now, whatever the setting.

The alerts are raised by the session hooks of the session feed, inside the hook
last, within a 4 second budget for the whole hook, so a notification program
that hangs cannot stall claude.
A session that is idle and waits for its next prompt (claude's `idle_prompt`)
does not notify unless you turn that on with `clalt notify idle on`; it is off by
default, and `clalt notify idle off` turns it off again.
