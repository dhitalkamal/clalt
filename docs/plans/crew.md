# clalt v2 step 6: Kiro Crew gateways per account

Drafted 2026-10-23 for review. Implements step 6 of docs/plans/clalt-v2.md. Branch
feat/crew, one PR against develop.

## Scope

In:

1. `clalt crew start|stop|status|logs <account>`, and `clalt crew status` for every
   gateway clalt knows.
2. `start` runs one detached Kiro Crew gateway for the account: the account's folder as
   CLAUDE_CONFIG_DIR, auth env vars stripped, its own Kiro Crew home and port, and the
   defaults below. It checks the Kiro Crew version first and refuses one outside the
   supported range.
3. The account must exist, not be hidden and be signed in, the same check `clalt run`
   makes. Nothing else is started for an account that fails it.
4. A crew section in `clalt doctor`: stale records and a version out of range.
5. A provider function for the dashboard: the gateways with account, port and state.
   Wiring it into the dashboard is a later pass.
6. docs/crew.md, the command table, the README list and the CHANGELOG.

Out, on purpose:

- Anything that touches a gateway clalt did not start, such as the scratch gateway on
  port 5477. clalt only knows gateways from its own records.
- A way to open a gateway to the network. Gateways are localhost only. A per gateway
  opt-in is an open question below.
- A switch for Kiro's steering injection (no supported switch exists, see the facts).
- A forced stop, auto start at login, `clalt crew open` (a dashboard link needs a
  token, which clalt never prints), Windows, and the plugin that picks an account per
  session inside one gateway.

## Facts

Checked on this Mac on 2026-10-23 against Kiro Crew 0.9.0 (`kirocrew --version`). Only
`--help`, `--version`, reading files and read-only `ps` were used. Nothing was started,
stopped or configured, and no token or key file was opened. "Docs" are the files in
the app bundle: /Applications/KiroCrew.app/Contents/Resources/backend-dist/
kirocrew-backend-arm64/lib/python3.12/site-packages/kiro_crew/docs, and "source" is the
python next to them.

Version
- The installed Kiro Crew is 0.9.0. The notes of the 2026-10-19 spike say 0.7.2 (the
  doctor output of that run says `kiro_crew: 0.7.2`), so the app was updated since.
  What the spike proved was proved on 0.7.2. Nothing below was proved by a real start
  of 0.9.0.
- `kirocrew --version` prints `kirocrew 0.9.0`, exits 0, and creates nothing in the
  data home it is given (tried with a home that did not exist).
- The changelog in the bundle has 0.7.2 (2026-09-28), 0.8.0 (2026-10-08) and 0.9.0
  (2026-10-09).

The process
- ~/.local/bin/kirocrew is a link to a bash launcher in the bundle. It ends in
  `exec python3.12 -s -P -m kiro_crew "$@"`, so the pid of the process clalt starts is
  the pid of the gateway. The running scratch gateway looks the same in `ps`:
  `python3.12 -s -m kiro_crew gateway --port 5477 --no-open --no-tunnel`.
- `kirocrew gateway --help`: `--port PORT|auto`, `--no-open` (no browser),
  `--no-tunnel` (never publish a tunnel), `--json-ready` (prints a line with the token,
  never used here), `--approval {reads,yolo,interactive}` (never used here).
- The gateway holds an exclusive flock on <home>/gateway.lock for its whole life, and
  a second gateway on the same home is refused (source: gateway_lock.py).
- Shutdown budget: 10 seconds for a cooperative stop plus 10 of margin, after which
  the service managers use SIGKILL (source: gateway_shutdown_budget.py).
- `kirocrew stop --port P --expect-pid N` stops only if N is the sole listener on P and
  holds the home's gateway.lock. A bare `kirocrew stop` or `restart` may go to a
  launchd service, so clalt never runs either.

Settings (docs/configuration.md, source: config/sections.py, cli_config.py)
- Environment: KIROCREW_HOME (data dir), KIROCREW_PORT, KIROCREW_WORKSPACE (workspace
  root), KIROCREW_SKIP_MODEL_DOWNLOAD=1 (skip the 640 MB embedding model),
  KIROCREW_CC_PERMISSION_MODE=auto (changes approvals, so an inherited one is removed).
- The workspace root defaults to a shared place (macOS /Volumes/workplace/
  kirocrew-workspace, else ~/workplace/kirocrew-workspace), so two gateways would share
  it unless KIROCREW_WORKSPACE is set.
- Bind address: `dashboard.url` empty means localhost only, and `kirocrew --help`
  says the dashboard binds loopback by default.
- `agent.acp_backend` (default "") and `agent.member_acp_backend` (default "kas") both
  take "claude". `agent.role_models` is a map with the keys `background` and `subagent`.
  `agent.approval_mode` is "auto" or "interactive", default "auto".
- `kirocrew config set KEY VALUE` reads the value as true, false, a number, json, else a
  string, writes <home>/config.json under a lock, and logs the change in the audit log.
- Updates: the desktop app's bundled gateway does not update itself.
- KIRO_HOME: the private layout Kiro Crew accepts is <data home>/kiro (agent.py,
  `_decline_shared_agent_home`). The docs warn that KIRO_HOME alone does not isolate
  the kiro-cli backend, which this design does not use.

Steering (source: context.py `_load_steering_resources`, agent.py)
- On the claude backend the gateway reads <KIRO_HOME>/agents/kirocrew.json and globs
  its `file://` resources against $HOME. The shipped resource is
  `file://.kiro/steering/**/*.md`, so ~/.kiro/steering is injected into every session.
  A private KIRO_HOME does not change that: the scratch gateway's own spec has the same
  resource. A non-empty `resources` list in an existing spec survives a rebuild, but
  no config key and no `kirocrew agent` command sets it. Rewriting Kiro's spec files
  from clalt is not a supported way, and those files can hold credentials.

Earlier spike (2026-10-19, 0.7.2, a real run)
- gateway -> claude-agent-acp 0.93.0 -> the real claude on the account's login.
- One gateway per account through CLAUDE_CONFIG_DIR works. A folder with no login gives
  "not signed in" and no state leaks between gateways.
- The spike config held acp_backend claude, member_acp_backend claude and role_models
  background and subagent both "haiku". "haiku" is the model id that worked.
- Known gaps, unchanged from the v2 plan: no reasoning text, approvals that claude's own
  settings already allow skip Kiro's gate, and the opus and sonnet picker fails.

Not verified
- A real start of 0.9.0 with these defaults, the bind address after a start, and a
  `config set` on a home that has never run a gateway (the spike did it before its first
  start: config.json.bak is older than the first gateway log).

## Data formats

All under ~/.config/clalt/crew, owned by clalt. The account folder belongs to claude and
holds nothing of this.

    crew/                        0700
      .lock                      one lock for start and stop
      <account id>/              0700, one per account
        gateway.json             the record, 0600, removed by stop
        gateway.out              stdout and stderr of the current run, 0600
        gateway.out.prev         the run before it
        home/                    KIROCREW_HOME of this gateway
          config.local.json      the pinned defaults (clalt writes, merging)
          kiro/                  KIRO_HOME
          workspace/             KIROCREW_WORKSPACE

gateway.json:

    {"account": "work", "pid": 4242, "port": 5500,
     "home": "/abs/path/home", "version": "0.9.0",
     "startedAt": "2026-10-23T15:05:12Z",
     "procStart": "Thu Oct 23 15:05:12 2026"}

`procStart` is what `ps -o lstart=` printed for the pid right after the launch. It is kept
as text and only compared, never parsed, so the locale and the zone cannot matter.

`status --json` prints one object per gateway: account (the label), id, state, port,
url, pid, version, startedAt, uptimeSeconds, home, log. States: running, stale (the
process is gone, or the pid belongs to another process now), unverified (alive and
started when the gateway did, but the command line is not a gateway for that port),
stopped (only for a named account that has no record).

## Defaults

Started as `kirocrew gateway --port N --no-open --no-tunnel`, with:

| what | value | why |
| --- | --- | --- |
| CLAUDE_CONFIG_DIR | the account folder | the point of the feature |
| auth env vars | stripped, names noted on stderr | a stray key would outrank the login |
| KIROCREW_* and KIRO_HOME | removed, then set below | an inherited home would be somebody else's |
| KIROCREW_HOME | <crew>/<id>/home | one data home per gateway |
| KIRO_HOME | <home>/kiro | the private layout Kiro Crew accepts |
| KIROCREW_WORKSPACE | <home>/workspace | the default is shared |
| KIROCREW_PORT | the allocated port | what its helper processes read |
| KIROCREW_BIND | 127.0.0.1 | localhost only, stated and checked |
| CLAUDE_CODE_EXECUTABLE | the resolved claude | the adapter runs the same claude as clalt |
| channel credentials | SLACK_, TELEGRAM_, DISCORD_, WEBEX_, WECOM_, FEISHU_, JIRA_ variables removed | not for an account's gateway |
| KIROCREW_SKIP_MODEL_DOWNLOAD | 1 | no 640 MB download per account |
| agent.acp_backend, member_acp_backend | claude | from the v2 plan |
| agent.role_models | background and subagent "haiku" | from the v2 plan |
| agent.approval_mode | interactive | not auto-approving is the safe unknown |
| bind address | localhost, left at Kiro's default | decided by the user |

The agent.* values are pinned in <home>/config.local.json (Kiro Crew's overlay, whose
values win over config.json) on every start, merging into what the file has.

## Decisions

1. One lock (crew/.lock) for start and stop, because ports are allocated across all
   gateways. It is released before the wait for the gateway to answer.
2. Port: the first free one from 5500 to 5599. Skipped: ports in another record, ports
   something listens on (127.0.0.1 and ::1), and always 5476 to 5478 and 9100 to 9199
   (Kiro's default, scratch gateways, Kiro's app backends), whatever range is given.
3. Detached with its own session (setsid), stdin from /dev/null, output appended to
   gateway.out. A launched gateway outlives clalt and the terminal.
4. Ready means the port answers on 127.0.0.1. A running gateway that `start` finds also
   gets this wait. Once it answers, lsof must show listeners on loopback only; one that
   listens elsewhere is stopped and reported, and an lsof that cannot say is a warning. `start` waits up to 30 seconds. A gateway
   that dies meanwhile is a failure, with the last lines of its output, and its record
   is removed. One that is alive but silent is a warning, not a failure: a first start
   builds databases.
5. Stop signals only a process that passes all of: the record names its pid, `ps` shows
   the same start text as at launch, and its command line names a Kiro Crew `gateway`
   with `--port` equal to the record, where the program is the launcher (kirocrew, or
   the script a shell or python was given, or `-m kiro_crew`) and `gateway` comes right
   after it. A pid that is gone, or whose start text differs,
   removes the stale record and signals nothing. A pid that is alive with the same start
   text but another command line is left alone and reported. If `ps` cannot be read,
   nothing is signalled. Never by name, never by port alone.
6. Stop sends SIGTERM to the process group when the pid leads its own (it does, it was
   started with setsid), so the claude adapters go with it, and waits up to 30 seconds
   (Kiro's budget is 20). It does not escalate to SIGKILL.
7. Only the 0.9.x series is supported, and a version with anything after its digits
   (0.9.0rc1) is refused. 0.9.0 is what this plan read. The earlier spike ran 0.7.2, but
   0.8 was never looked at. CLALT_CREW_ALLOW_UNTESTED=1 starts any version that can be
   read. The doctor message says so.
8. The defaults are written into config.local.json by clalt itself, merged and atomic,
   keeping every other key. `kirocrew config set` is not run, and Kiro's config.json and
   spec files are not written. A file that is not json stops the start.
9. The doctor says nothing about crew until a record exists, so people who never use it
   see no new lines and doctor does not run kirocrew for them.
10. `status` and `logs` never print a token. `logs` also masks `token=` values in what
    it shows, and gateway.out is 0600. `--json-ready` is never passed.
11. Everything started or looked at goes through one injected Runner: kirocrew, and `ps`
    for identity. Tests use a fake Runner, and the CLI tests use a fake kirocrew script
    named by CLALT_KIROCREW_BIN. The cli test binary points that variable at a path
    that does not exist, so a test that forgets the fake cannot reach the real one.
12. Unix only. On Windows `clalt crew` says it is not available, and the package still
    compiles there (process code sits behind build tags).

## PR plan

One PR, feat/crew into develop, commits in this order, each with its tests:

1. this plan
2. paths, record, version range, port allocation
3. the runner, process identity, and the environment
4. start, with seeding and the readiness wait
5. stop, status and the dashboard provider
6. logs
7. the `clalt crew` commands
8. doctor
9. docs, README, CHANGELOG

## Left for the wiring pass

- Call `crew.Gateways(ctx)` from the dashboard Source and show account, port and state.
- Dashboard actions for start and stop, through the same Manager.

## Open questions

1. Steering: nothing switches it off. Keep it injected, move ~/.kiro/steering aside, or
   ask upstream for a config key? The plan default "Kiro steering not injected" is not met.
2. Whether to run a real start of 0.9.0 before the PR merges.
3. Two defaults were added to the list: agent.approval_mode interactive and
   KIROCREW_SKIP_MODEL_DOWNLOAD=1. Keep them?
4. Kiro data lives under ~/.config/clalt/crew, next to clalt's other state. It can be
   large (memory, sessions). Move it to the XDG data dir instead?
5. How a user opens one gateway to the network: a start flag that sets `dashboard.url`
   in that gateway's home is the likely shape. Not built, because binding was not seen.
6. Opening the dashboard needs a token: `KIROCREW_HOME=<home> kirocrew token --port N`
   prints a link. A `clalt crew open` could run it. Wanted?
7. `claude-agent-acp` is not bundled with Kiro Crew and clalt does not check for it.
