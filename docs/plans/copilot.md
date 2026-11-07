# GitHub Copilot CLI in the office

Status: planned 2026-11-07. Copilot only for now, Codex and Gemini are parked.

## Scope

Copilot CLI sessions show in the office next to claude sessions, in one view,
with the same parity claude sessions have: live state (busy, idle, needs you,
ended), subagents, activity, the wave and the desktop notification.

Observe only. clalt never runs a Copilot login, never switches Copilot
accounts, never reads Copilot tokens and shows no Copilot usage. Copilot
sessions sit in their own team with a fixed colour and the label Copilot.

Out of scope here: agent to agent messaging between claude and Copilot (a later
phase with its own plan, safety review and terms check), Codex, Gemini, and the
dark and graveyard themes.

## Why observe only

Checked 2026-11-07 against the GitHub Terms of Service (effective 2026-04-27),
the Acceptable Use Policies and the Copilot CLI license. One person may keep no
more than one free GitHub account, a login may not be shared, and API tokens
may not be shared to exceed rate limits. Nothing restricts reading the CLI's
own session files or using its documented hooks. The CLI already ships its own
account switching (/user switch), so clalt adds little there.

## Data sources

1. An opt-in hook file, ~/.copilot/hooks/clalt.json ($COPILOT_HOME/hooks when
   set), written only by clalt copilot hooks on and removed by off. It runs
   clalt hook copilot <event> and feeds the same feed store claude hooks feed.
2. The session folders ~/.copilot/session-state/<id>/ with events.jsonl and
   workspace.yaml, for sessions that began before the hook was on.

~/.copilot is shared with the IntelliJ Copilot plugin and other tools (another
tool's hooks/orca.json is already there). clalt touches only its own
clalt.json, and never opens config.json (auth state), providers.json (may hold
API keys), mcp-config.json or permissions-config.json.

## Contract

feed:
- SessionState gains tool (string, omitempty). Missing means claude.
- Copilot sessions use the handle cp-<copilot session id>, same id rules as
  ext handles (letters, digits, hyphens, at most 60). CopilotHandle, IsCopilot,
  ValidKey accepts it.

office snapshot (version stays 1, the fields are additive):
- accounts[].tool: "claude" or "copilot".
- sessions[].tool: "claude" or "copilot".
- the Copilot team is the account id "~copilot", label "Copilot", tool
  "copilot", with no limits and no forecast. It is listed only while at least
  one Copilot session is shown, and always after the claude accounts so their
  colours do not move. "~" can never be in a clalt account name
  (internal/account/validate.go), so the id cannot clash.
- Copilot sessions have no actions; why says clalt only shows them.

mcp: sessions gains tool. limits and best_account ignore non-claude accounts.

page: the Copilot team gets a fixed colour outside the claude palette, shows
the label Copilot as text (not colour only), and shows no 5h or 7d bars.

## Hook mapping (PR 2, confirm against real payloads first)

- sessionStart: starting
- userPromptSubmitted, preToolUse, postToolUse: busy, with the tool name only
- permissionRequest, notification of type permission_prompt: needs you
- agentStop: idle
- subagentStart, subagentStop: subagent running, done
- sessionEnd: ended

The hook is observe only: it never prints a permission decision, always exits
0, keeps to a fixed time budget, and keeps only state, cwd, tool name and ids.
Never prompt text, command contents or file contents.

## Threats

- a hook that answers permissionRequest could approve commands: ours prints
  nothing and exits 0.
- a slow hook stalls Copilot: fixed time budget, like the claude hook.
- the payload is untrusted text from the repo and the model: cleaned and cut
  like any feed field, unknown events ignored.
- other tools' hook files and Copilot auth files: never opened or changed.

## PRs

1. groundwork: tool field through feed, office snapshot, mcp and page, the
   Copilot team and its placement, tested on fake data.
2. hook: clalt hook copilot and clalt copilot hooks on, off, status, wired into
   the needs-you notification.
3. session reader: events.jsonl and workspace.yaml for sessions, subagents and
   activity.

## Signals

clalt copilot hooks status says whether the hook file is installed and when the
last Copilot event reached the feed. Hook failures go to the office log.

## Observed format

From a real session (Copilot CLI 1.0.83): events.jsonl lines have type, data,
id, timestamp, parentId. Seen types: session.start (data.sessionId,
data.context.cwd, data.copilotVersion), session.model_change, session.info,
hook.start and hook.end (data.hookType, data.input), session.shutdown.
workspace.yaml has id, cwd, client_name, created_at, updated_at. Tool calls,
permission prompts and subagents still need a captured session.
