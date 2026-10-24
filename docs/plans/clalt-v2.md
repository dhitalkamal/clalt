# clalt v2 plan

Approved 2026-10-19. Builds on PR #6 (dashboard v2 on demo data, merged). Replaces
the PR 2 and PR 3 sections of docs/plans/dashboard-v2.md.

## Goal

Run and manage many claude sessions at once across several accounts, see them and
reach them, and move a conversation to another account mid-task.

## Verified on this Mac (claude 2.1.296)

- CLAUDE_CONFIG_DIR keys claude's login to that directory (claude docs).
- claude keeps the account identity (email, org, plan type) in <config dir>/.claude.json.
  A login slot set only through CLAUDE_SECURESTORAGE_CONFIG_DIR shares ~/.claude.json, so
  every slot shows whoever signed in last. Rejected for parallel accounts.
- A thin profile works. It is its own CLAUDE_CONFIG_DIR with projects/ linked to
  ~/.claude/projects. It keeps its own identity, and `claude --resume <id>` continues a
  conversation started under another account.
- A profile with no login refuses to run ("Not logged in"). There is no silent fallback
  to another login.

## Decisions

### Accounts: thin profiles managed by clalt

- Each account is a folder, ~/.config/clalt/accounts/<id>, used as CLAUDE_CONFIG_DIR.
- What the folder owns: its claude login (a Keychain entry keyed by the folder path) and
  .claude.json (identity and per-account state). Everything else links to ~/.claude:
  projects, settings, CLAUDE.md, skills, agents, commands, plugins. The exact list is
  fixed at the step 2 kickoff, see docs/plans/thin-profiles.md.
- The id is stable, the label is display only, and the folder never moves.
- `clalt login <name>` creates the folder and runs `claude auth login` with it.
- Migration: transcripts in the old ~/.config/clalt/profiles/<name>/projects are copied,
  not moved, into ~/.claude/projects first. The old folders stay until the user removes
  them.
- The vault, the keepalive job and the supervisor are retired. Their files stay on disk
  until a later cleanup.

### Terminal sessions: the clalt run wrapper

- `clalt run [account]`:
  - starts claude in the current terminal with the account's CLAUDE_CONFIG_DIR;
  - strips env tokens;
  - picks the allowed account with the most room when none is given;
  - registers the session: account, folder, and terminal location (kitty window id,
    tmux pane, or IDE).
- Move: on request (dashboard or `clalt move`), the wrapper waits for a pause between
  replies (Stop hook), ends claude, and resumes the same session id under the target
  account in the same terminal.
- Auto-switch: when the session's account crosses the limit threshold, or claude reports
  the limit, the wrapper moves at the next pause. The target is the allowed account with
  the most room, and the wrapper prints one line saying so.
- Account rules: per account, the folders it may serve (work or personal) and whether
  auto-switch may use it. Auto-switch never crosses a rule.
- Feed: claude hooks plus the statusline, installed per profile, report session state
  and limits. claude's own ~/.claude/sessions/<pid>.json is read where it helps; it is
  undocumented, so it is checked at runtime.
- Extras:
  - needs-you alerts (a desktop notification and the dashboard);
  - resume anything (every conversation, across accounts);
  - launch into a new kitty tab or tmux window;
  - jump to a session (kitty remote control on a socket only, or tmux; an IDE only shows
    the location).

### Dashboard

PR #6's Source and Actions ports get real adapters for:
- profiles;
- `claude auth status --json`;
- the feed;
- the usage ledger.

## Build order

1. Land the useful parked fixes as small PRs, from the 2026-10-19 triage:
   - fix/run-output-race (03a46e9)
   - chore/fs-helpers (8a6adbc)
   - fix/usage-dedup-pricing (0cd36a8, ee6070b)
   - fix/account-name-validation (151158f)
   - chore/release-ci: release-tooling minus bd54bc8 and 9af38c7, with the CHANGELOG
     trimmed to what ships. After the race fix.
   - feat/usage-session-attribution (fb7b41c, ba0d24e). After fs-helpers and dedup.
2. Thin profiles, the history copy, and `clalt login` and `clalt run` on profiles. Retire
   the vault, keepalive and supervisor.
3. Session wrapper; the feed (office-feed core, statusline install, run-signals adapted
   to profiles); the dashboard on real data.
4. Move, auto-switch, account rules.
5. Needs-you alerts, resume anything, launch and jump.
6. Kiro Crew gateways: dropped on 2026-10-23

Each step gets its own kickoff plan block before code, test-first, in small PRs.

## Parked branches (triage 2026-10-19)

- Obsolete, kept: fix/hardening, fix/keepalive-live-sessions, chore/merge-trial,
  feat/tui-redesign, feat/clalt-lens, feat/office-handoff.
- Parked: feat/office-server, feat/office-page, and feat/clalt-office apart from the
  statusline install.
- Landed in step 3: the office-feed core, statusline install, and run-signals.

## Risks

- ~/.claude/sessions/<pid>.json is undocumented. The design treats it as optional and
  checks it at runtime.
- A moved conversation re-reads its whole context on the new account (no shared prompt
  cache), which costs part of that account's limit.
- Auto-switch moves work between accounts. Account rules and a visible notice are
  required, and each account's terms should be checked before auto-switch is enabled.
- bubbletea v2.1.0 can stall its renderer when an exec returns at once. Resolve claude
  with exec.LookPath before returning a command.
