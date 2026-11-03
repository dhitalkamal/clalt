# MCP server and Claude Code plugin

`clalt mcp` is a read-only MCP server. It lets you ask Claude Code things like
"which account has the most room right now?" or "what did I do today?" and get
the answer from your own clalt data.

It speaks JSON-RPC over stdin and stdout (the MCP stdio transport), opens no
port, and changes nothing: no account, session or file of claude is touched,
and there is no tool that writes or acts.

## Tools

Each tool returns two text items: a one-line summary for people, then compact
JSON for the model.

- `limits`: per account, the 5h and 7d usage percent (`usedPct`, `leftPct`),
  when each window resets (`resetsAt`, `resetsIn`), and the forecast of when the
  5h window fills at the current pace (`forecast.fiveHourAt`, null when there is
  no live session, no recent reading or the window resets first). An account
  with no reading has `fiveHour` and `sevenDay` null, which is not 0 percent.
- `best_account`: the account with the most 5h room now, and `why`. An account
  with no 5h reading, or with its 7d window used up, is not picked. Ties go to
  the lighter week. `best` is null when nothing qualifies.
- `sessions`: running sessions with account, project folder name, model, state
  (`busy`, `idle`, `needs-you`, `starting`), the needs-you reason, and subagent
  counts (`total`, `running`). Prompt text and subagent labels are left out.
- `today`: what each account and project did on a day: sessions, tokens, cost,
  active minutes, tool calls, failure count and the five most used tools. Takes
  an optional `day` as `YYYY-MM-DD` (local day, today when left out). File
  names and failure text are left out.

A tool that fails answers with `isError: true` and a short message, not a
crash. Paths and error details are never put in the message.

## Privacy

- Only the data `clalt office` already shows is read: the registry and the
  live feed (limits and session states), and the transcripts for the day
  summary.
- Credential, keychain and token files are never read.
- Email addresses are never printed: an account is named by its label, and a
  label with an at sign is cut there.
- Nothing leaves the machine except what Claude Code sends to the model as the
  tool result, so the answer text above is what the model sees.

## Install as a Claude Code plugin

clalt must be on your PATH (`make install` puts it in `~/bin`; check with
`clalt version`). The plugin only declares the server command `clalt mcp`.

In Claude Code:

```
/plugin marketplace add dhitalkamal/clalt
/plugin install clalt@clalt
```

The plugin lives in [plugin/](../plugin) and the marketplace file is
[.claude-plugin/marketplace.json](../.claude-plugin/marketplace.json). The
tools show up as `mcp__plugin_clalt_clalt__limits` and so on.

## Add the server by hand instead

```
claude mcp add --transport stdio --scope user clalt -- clalt mcp
```

Use `--scope project` to write it to the project's `.mcp.json` instead. The
tools are then named `mcp__clalt__limits` and so on.

## Try it without Claude

```
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"limits","arguments":{}}}' | clalt mcp
```

## Protocol notes

One JSON message per line, UTF-8, replies on stdout, logs on stderr. It answers
`initialize` (echoing the client's protocol version when it is 2024-11-05,
2025-03-26, 2025-06-18 or 2025-11-25, else the latest), `ping`, `tools/list`
and `tools/call`. It also answers `server/discover` and accepts the per-request
metadata of the 2026-07-28 revision, so a client that probes for it works.
Batches are refused, as the current revisions dropped them.
Other methods get `-32601`, and notifications are never answered.
