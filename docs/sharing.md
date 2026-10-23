# Sharing with ~/.claude

Every account folder is linked to the shared claude home, `~/.claude`, so every
account sees the same settings, skills, plugins, commands, agents, CLAUDE.md and
the same conversation history. `clalt login` and `clalt run` sync the links, and
`clalt doctor` reports them without changing anything.

## The shared home

The shared claude home has to exist. clalt never makes it, because it may only
be unmounted: if `~/.claude` (or `$CLAUDE_CONFIG_DIR`) is missing, `login` and
`run` stop and say so. Run claude once, or make the folder, and try again.

A home that exists without a `projects` folder, because claude ran and saved
nothing yet, gets one: `login` and `run` make `<shared home>/projects` (mode
0700), since without it the history of the account would stay with the account.
A `projects` that is there in any form counts, a link into a volume that is not
mounted included. That one is for `run` to report (see "The history check").
`clalt doctor` does not make the folder, and says that `run` will.

## What is shared

- Folders are linked by default, so a new folder you add shows up in every
  account on the next run. Kept per account, never linked: `sessions`,
  `daemon`, `jobs`, `backups`, `cache`, `statsig`, `telemetry`, `downloads`.
- `skills` and `plugins` are linked entry by entry, not as whole folders. The
  account gets a real `skills/` and `plugins/` folder and every entry of the
  shared one is linked into it, except `synced` and `.trash` (and
  `plugins/known_marketplaces_claudeai.json`), which stay per account because
  claude.ai sessions download that account's own skills, plugins and
  marketplaces there. A link to the whole folder from an earlier layout is
  replaced by a real folder with links inside; only the link goes, so nothing is
  lost.
- Loose files are linked only from an allow-list: `settings.json`,
  `settings.local.json`, `CLAUDE.md`, `keybindings.json`, `history.jsonl`.
  claude keeps per-account state in other loose files, so those stay per
  account.
- `.credentials.json` and `.claude.json` are never linked.
- A missing link is created, a link that points somewhere else is re-pointed
  (the new link is made under a temporary name and renamed over the old one; on
  Windows a rename cannot replace a link to a folder, so there the old link is
  removed and the new one made right after, and if that fails the name is
  missing until the next sync makes it again), a link whose source is gone is
  removed. A real file where a link belongs is kept
  and reported with the reason (claude may replace a link with a file when it
  saves), and so is a folder with anything in it; an empty folder holds nothing,
  so the link replaces it. An entry of the shared home that cannot be read is
  kept and reported, never taken for gone, and so is an account's link to an
  entry that is itself a link to something that is missing (a volume that is not
  mounted). `clalt doctor` lists what was kept. Nothing is ever copied in place
  of a link.
- When the shared `skills` or `plugins` is gone altogether, only the links the
  account made into it are removed. Credential files are never linked out of
  these folders either.
- One sync runs at a time per account (a lock file, `.clalt-sync.lock`, in the
  account folder).
- If `CLAUDE_CONFIG_DIR` is set to a folder outside clalt's own, that folder is
  the shared home instead, and its `.claude.json` is the shared global config.

## The history check

Just before it starts claude it looks at `projects` in the account folder: it
has to be a link that reaches the shared history folder (`~/.claude/projects`).
claude makes a real `projects` folder when there is none, and the conversations
of that account would then be the only ones that account could resume. So `run`
refuses, and points at `clalt doctor`, when the account has a real `projects`
folder of its own, or when the shared one is not reachable: it is on a volume
that is not mounted, or the link aims somewhere else. That holds for an account
made while the volume was unmounted too, which has no link at all. Mount the
volume and run again.

`clalt login` looks at the same thing right after it has linked the account,
before the sign-in starts. It does not refuse, the sign-in goes on, but it prints
one warning line that says `run` will refuse until the history is reachable, so
that the first `run` is not where you find out.

## MCP servers

User-scope MCP servers (the top-level `mcpServers` of the shared
`~/.claude.json`) are mirrored into the account's own `.claude.json` on every
run. A server in the shared config is added or updated; one clalt mirrored
earlier that has left the shared config is removed; one the account added
itself is kept. clalt remembers which names it mirrored in `.clalt-mcp.json`
inside the account folder. Everything else in the account's `.claude.json` is
left as it is, and server definitions are never printed (their env blocks can
hold secrets), only names. A shared server that replaces one the account added
itself under the same name is reported by name. If the shared `~/.claude.json`
is gone while servers were mirrored from it, the sync stops with a warning and
changes nothing: that is more likely an unmounted home than a decision to drop
every server.
