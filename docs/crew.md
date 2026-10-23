# Kiro Crew gateways

`clalt crew` runs one Kiro Crew gateway per account, with that account's claude
login. It needs Kiro Crew installed (`kirocrew` on the PATH, or CLALT_KIROCREW_BIN)
and the `claude-agent-acp` adapter, which Kiro Crew does not bundle. Not available
on Windows. What was checked about Kiro Crew, and why each default is what it is, is
in [plans/crew.md](plans/crew.md).

## Commands

```
clalt crew start <name>     run a gateway for the account
clalt crew stop <name>      stop the gateway clalt started for it
clalt crew status [name]    port, state, version and uptime (--json)
clalt crew logs <name>      the end of what the gateway printed (-n 100)
```

`start` needs the account to exist, not be removed, share the history and be signed
in as the recorded email, the same as `clalt run`. It then checks the Kiro Crew
version (only 0.9.x, no suffix; CLALT_CREW_ALLOW_UNTESTED=1 lifts it), picks the first
free port from 5500 (5476 to 5478 and 9100 to 9199 are never used),
and starts the gateway detached, so it keeps running after the terminal closes. It
waits up to 30 seconds for 127.0.0.1:port to answer, then checks with lsof that it
listens on loopback only. A gateway that listens elsewhere is stopped. A gateway that exits meanwhile is an
error, with the end of its output; one that is alive but silent is a warning.

## What a gateway gets

- `CLAUDE_CONFIG_DIR` set to the account folder, and the variables that would
  override the login removed, as for `run` (see [auth.md](auth.md)). Inherited
  `KIROCREW_*` variables and `KIRO_HOME` and channel credentials (`SLACK_`, `TELEGRAM_`, `DISCORD_`, `WEBEX_`,
  `WECOM_`, `FEISHU_`, `JIRA_`) are removed too, and named on stderr.
- Its own data home, `~/.config/clalt/crew/<id>/home`, its own workspace and port.
- Pinned in `config.local.json` of its home on every start, other keys kept:
  `agent.acp_backend` and
  `agent.member_acp_backend` set to claude, `agent.role_models` background and
  subagent to haiku, and `agent.approval_mode` interactive. Kiro's own
  config files are not touched, and `kirocrew config set` is not run.
- `--no-open --no-tunnel` and `KIROCREW_BIND=127.0.0.1`: no browser, no tunnel,
  loopback only. `CLAUDE_CODE_EXECUTABLE` is the claude clalt resolved.
- `KIROCREW_SKIP_MODEL_DOWNLOAD=1`, so no 640 MB model per account.

Not done: Kiro's own steering files (`~/.kiro/steering`) are still injected into every
session, because Kiro Crew has no switch for it. Tool calls that claude's own settings
already allow skip Kiro's approval gate.

## Tokens

clalt never prints or logs a gateway's token. `logs` also hides `token=` values in what
it shows. To open the dashboard, ask Kiro Crew for a link yourself:
`KIROCREW_HOME=<home> kirocrew token --port <port>`, with the home and port from
`clalt crew status <name>`.

## Stopping

`stop` signals only the process in its own record, and only when `ps` shows the same
start time as at the launch and a command line that is still a Kiro Crew gateway on
that port. Never by name or by port alone, and never a gateway clalt did not start. It
sends SIGTERM to the gateway's process group, so the claude adapters end with it, and
waits up to 30 seconds; it does not kill. A pid that is gone, or that
another process has now, only loses the record. One that cannot be confirmed is left
alone and reported.

## Files

Under `~/.config/clalt/crew/<id>/`: `gateway.json` (pid, port, home, version, start
time; removed by `stop`), `gateway.out` and `gateway.out.prev` (output of this and the
last run), and `home/` (which holds `config.local.json`). `clalt doctor` shows crew lines once a record
exists: stale records and a Kiro Crew version out of range are warnings.
