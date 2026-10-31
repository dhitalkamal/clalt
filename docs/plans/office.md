# clalt office plan

Builds on the session feed (PR #38 and earlier) and the three.js office demo
(throwaway spike, v6). `clalt office` shows the agents of every account as
characters in a live 3D office in the browser.

## Scope

- `clalt office [--port N] [--no-open]` starts a small web server on this
  machine and opens the page. Ctrl-C stops it.
- The page is a viewer. It polls a read-only snapshot about once a second and
  animates the changes. No buttons act on a session.
- One fixed default layout: a cabin per session in an account wing, research,
  open workspace, a mezzanine with a review room and a meeting room, a cafe and
  a play area where idle agents take breaks, a lobby with the door.
- Camera: 75 degree start view, 360 degree orbit, right drag pans, wheel zooms,
  buttons for top view, front view and back to the overview (Esc too).

## Decisions

- Watch only. No action endpoints, no POST. Every method but GET is refused.
- Assets are embedded with go:embed: the page, the JS modules, three.js r170
  (MIT), the CC0 Kenney Furniture Kit and Mini Characters GLBs. No CDN, no
  network at run time. CREDITS.txt and three.js's LICENSE ship in the embed.
- Subagents are shown from day one. They walk in when the feed lists them and
  walk out when they are done. The feed records them in the `Subagents` field
  of its session state (`subagents` in the session file: id, type, label,
  state running or done, startedAt, endedAt). The office reads that field
  through its own `SubagentSource` interface, with the feed-backed `Real`
  as the implementation.
- One default layout. Build mode (moving furniture, saving a layout) comes
  later and is listed at the end.
- Plain claude sessions that the feed knows (ext sessions) are included, like
  in the dashboard.

## Snapshot contract

`GET /api/snapshot` returns JSON, `Cache-Control: no-store`:

    {
      "v": 1,
      "accounts": [
        { "id": "work", "label": "work",
          "limits": {
            "fiveHour": { "usedPct": 41.5, "resetsAt": "2026-10-24T15:00:00Z" },
            "sevenDay": null,
            "readAt": "2026-10-24T12:01:09Z" } }
      ],
      "sessions": [
        { "handle": "0123456789abcdef", "account": "work",
          "project": "clalt", "model": "Opus 5.5",
          "state": "busy", "needsYouReason": "",
          "startedAt": "2026-10-24T11:00:00Z",
          "subagents": [
            { "id": "a1", "type": "Explore", "label": "map call sites",
              "state": "running", "startedAt": "2026-10-24T11:05:00Z",
              "endedAt": null,
              "doing": { "kind": "read", "tool": "Grep", "state": "running",
                         "at": "2026-10-24T11:06:00Z" } } ],
          "doing": { "kind": "run", "tool": "Bash", "state": "fail",
                     "at": "2026-10-24T11:06:10Z" } }
      ]
    }

- `v` is the contract version, 1. A field may be added without a bump, one may
  not be renamed or change meaning without one. The page ignores a higher `v`
  than it knows and shows a notice.
- `accounts` are the accounts the dashboard lists (not removed ones), ordered by
  label then id. `label` is the display label only. No email, org or plan.
- `limits.fiveHour` and `limits.sevenDay` are `null` when the feed has no reading
  of that window, which is not 0 percent. `readAt` is `null` when there is no
  reading at all. A window that has reset since the reading is reported as
  `null`, the same way the dashboard treats it.
- `sessions[].state` is the feed state: `starting`, `busy`, `idle`, `needs-you`
  or `ended`. The page treats `starting` as idle and `ended` as leaving.
- `project` is the base name of the session's working folder, never the full
  path. `model` is the model id as the feed has it, shortened for display by
  the page. `needsYouReason` is the notification type of the hook, a short
  token such as `permission_prompt`, empty otherwise.
- `subagents[].state` is `running` or `done`. `endedAt` is `null` while running.
  `type` is the agent type (such as `Explore`), `label` is a short description.
  The page places a subagent by its type.
- `doing` (on a session and on each running subagent) is the newest tool call
  of its transcript, or `null` when there is none or it is over 2 minutes old.
  `kind` is `read`, `edit`, `run`, `web` or `other` (from the tool name by a
  fixed table), `state` is `running`, `ok` or `fail`, `at` is when it started
  or finished. It carries no file name, command or text. Added without a bump.
- Sessions are ordered by start time, then handle. Subagents by start time, then
  id. The order is stable so the page does not shuffle.

## Security model

The server is on localhost but other pages in the user's browser can still reach
localhost, so it is not open to every request.

- Listens on `127.0.0.1` only, never `0.0.0.0` or a name. Random free port
  unless `--port` is given.
- A random 128 bit token per run, from crypto/rand, hex. Every request needs it,
  compared in constant time. The first page load carries it as `?t=<token>`,
  the server answers with a `SameSite=Strict; HttpOnly` cookie and redirects to
  the clean URL. Later requests use the cookie, or an `X-Clalt-Token` header.
- The Host header must be the listen address (`127.0.0.1:<port>` or
  `localhost:<port>`), or the request is refused with 403. That stops DNS
  rebinding, where a hostile name resolves to 127.0.0.1.
- Only GET (and HEAD). Everything else is 405.
- Only embedded files and `/api/snapshot` are served. Paths are cleaned and
  looked up in the embed.FS, anything else is 404. No directory listing, no
  file system access outside the embed.
- Strict Content-Security-Policy: `default-src 'none'`, script, style, image,
  connect and font from `'self'`, `frame-ancestors 'none'`, `base-uri 'none'`,
  `form-action 'none'`. Plus `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: no-referrer`, `Cross-Origin-Resource-Policy: same-origin`.
  No CORS headers are ever set.
- The snapshot carries no prompts, no transcript text, no tokens, no
  environment, no email. Labels only. Every string is cleaned (control and
  format characters removed, length capped) before it is encoded.
- Errors go to `<clalt dir>/logs/office.log`, one cleaned line each. Responses
  say nothing about the cause.
- The token is printed in the URL on the terminal, nowhere else. It is not
  written to a file or the log.

## Layout of internal/office

    internal/office/
      snapshot.go      Snapshot types, Build from the sources
      sources.go       the small interfaces: accounts, sessions, limits, subagents
      clean.go         string cleaning for the snapshot
      server.go        handler: token, host check, method, CSP, routes
      run.go           Options, Run: listen, print the URL, open, shut down
      open.go          browser opener per OS, injected in tests
      web.go           go:embed of web/
      web/             the page (index.html, js/, vendor/, assets/, CREDITS.txt)

The CLI side is `internal/cli/cmd_office.go` (flags, adapters from the registry,
feed and accounts to the sources) and the command table, help and docs.

The page is plain ES modules, no build step. Every JS file is under 300 lines.
`main.js` wires the loop, `sync.js` turns a snapshot into agents (spawn, change
state, walk out), `poll.js` fetches it. The scene modules come from the demo.

## What is left for build mode

- Editing the layout: drag furniture, add rooms, per-account decorations.
- Saving a layout per machine and loading it (a `layout.json` in the clalt dir).
- Actions from the page (jump to a session, stop one). They need a POST
  surface, a CSRF story and a second look at the token model, so not here.
- Click through to a session's details beyond the hover card.
- A snapshot push channel (server sent events) instead of polling.
