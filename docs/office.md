# The office

`clalt office` opens a live 3D office in your browser. Every session of every
account is a character at a desk, every subagent is a character too. It is a
viewer: nothing on the page starts, stops or changes a session.

    clalt office [--port N] [--no-open] [--template <id>]

It prints a link, opens it in your default browser, and runs until you press
Ctrl-C. `--no-open` only prints the link. `--port N` picks the port, by default
a free one. `--edit` is still accepted and does nothing: saving from the page no
longer needs it.

## Running it in the background

You do not need to keep a terminal open. The PR review watcher and the limit
warnings run as long as the office server runs, so a background server keeps
them going.

    clalt office on             start it in the background and open the browser
    clalt office open           open the running one again
    clalt office status [--link]  on or off, port, pid, uptime, version
    clalt office off            stop it
    clalt office autostart on|off|status   start it when you log in (macOS)

- `on` starts `clalt office serve` detached (its own session, no stdin) and
  opens the link. The port is 47321, or the next free one up to 47340. If it
  already runs, `on` only opens it. It is not available on Windows yet: run
  `clalt office` in a terminal there.
- `off` asks the server to stop (SIGTERM) and waits up to 5 seconds. It only
  signals a process that proved itself: the pid in the run file must answer a
  token-checked `GET /api/ping` on the saved port as the clalt office server
  with that same pid. A stale run file is removed and nothing is signalled.
- `open` says so and suggests `clalt office on` when it is off.
- `status` never prints the link unless you add `--link`, because the link
  carries the token. It also shows the last error line of the log.
- `clalt office` in the foreground still works as before. It refuses `--port N`
  when the background office already uses that port.

Files, all in `~/.config/clalt/office/` (the folder is 0700):

- `run.json` (0600): `pid`, `port`, `token`, `startedAt`, `version`. This is the
  only place the token is kept besides the browser cookie. It is removed when
  the server stops.
- `office.log`: output and errors of the background server, moved to
  `office.log.old` at about 1 MB (one old file is kept). The token is never
  written to it.

### Start at login

On macOS, Settings has a "Start office when I log in" checkbox, and
`clalt office autostart on|off|status` does the same from a terminal. It writes
`~/Library/LaunchAgents/com.dhitalkamal.clalt.office.plist` (runs the absolute
path of the clalt binary with `office serve`, no token in it) and loads it with
`launchctl bootstrap gui/<uid>`. Off runs `launchctl bootout` and removes the
file. If you move or reinstall clalt, run `autostart on` again so the path is
right. The server started by launchd writes `run.json` like any other, so
`clalt office open` works. Turning it off from the page while launchd runs that
very server removes the file but does not unload the job, so the page stays up
until the server stops. On other systems the checkbox is disabled.

## How it looks

- A glass cabin per session, grouped in a wing per account. The wing sign shows
  the account label and its 5h and 7d limit bars. A bar with a question mark is
  a window the feed has no reading of yet, which is not 0 percent. A line
  under it counts the sessions: working, needs you, idle, resting or on break.
- Each session and subagent stands on a small flat ring in its account colour,
  so the account reads at a glance. Team members have no ring.
- The sign over a cabin and the tag over a character show the project folder of
  the session. Subagents show their agent type.
- Subagents sit at desks by kind: Explore in Research, Plan in the Meeting room,
  reviewers and test agents in the Review room, everything else in the Open
  workspace. When a room is full the rest sit in the Open workspace. Team members
  too, interns beside their member.
- A busy session types, with a spinning ring over its head. An idle one gets up
  now and then and walks to the cafe or the play area, chats, plays ping-pong or
  the arcade, and comes back (you can change that: [idle choices](office-idle.md)).
  A session that needs you stands at its cabin door and waves with a bubble.
- A new session or subagent walks in through the lobby. A finished subagent
  hands a paper to its main agent and leaves. An ended session walks out.
- With nothing running the office is empty and says so.

## Controls

- Sky camera, the default: drag turns the camera around the spot under the
  cursor, all the way round and from straight down to almost level. Shift and
  drag (or right drag) moves the view, wheel zooms to the cursor, two fingers
  pinch on a touch screen. W A S D or the arrows move (Shift is faster), Q and E
  turn, Z and X tilt, R and C go higher and lower, double-click flies there.
  The view stays over the walkable area of the template (the hall floor, the island): it
  cannot zoom out past the point where that area fills the screen, a low camera stays inside
  the hall walls, and in the hall the walls between the camera and the room fade out.
- G goes down to ground level and walks (W A S D, drag to look, Shift runs,
  Space jumps, not on the stairs). The third person camera pulls in when a wall,
  tree or deck edge is in the way. V switches between first and third person.
  F follows the agent under the cursor or the selected one. Esc leaves ground
  or follow, and from the sky goes to the overview, like Home.
- P turns auto spin on and off and L the labels. Old links with
  `#cam=top`, `plan`, `iso` or `hero` open the sky camera at the overview.
  The keys do nothing while you edit.
- The camera bar and these keys work the same in every template, the hall
  included. The Overview button goes back to the overview.
- Hover a character for a card with the handle, project, model, state and
  account label. Click a room for its agents; its sign counts the working ones.
- Team lines draws dotted lines from each subagent to its main agent. Tags shows
  every tag, not only the ones of the main agents.

## Battery

The page draws at 60 frames per second only while you drag, zoom, walk or the
camera flies, at about 15 while agents move or type, and about 2 when nothing
moves. After 2 minutes without input it stays at the slow rate until the mouse
or a key wakes it. Shadows of the scenery are baked once per layout, agents get
a soft disc on the floor, and the pixel ratio is capped at 1.5. Shift+F shows
fps, draw calls, triangles and frame time. To measure the real power on a Mac,
open the office tab alone, leave the mouse still, and run
`sudo powermetrics --samplers gpu_power,cpu_power -i 1000 -n 30` for 30
seconds, once on the old build and once on this one, and compare the GPU power
and CPU power lines (combined power in mW is the number to compare).

## Templates

The office starts in the hall, one building with a room for each kind of work.
The other four templates are garden campuses: pavilions on a lawn with paths,
water and trees. There are five templates:

| id        | what it is                                                       |
| --------- | ---------------------------------------------------------------- |
| `rows`    | cabins in rows along the back                                    |
| `village` | pavilions on a ring around a green with a pond and the cafe      |
| `river`   | cabins along the west bank of a river                            |
| `street`  | one winding street from the gate to a fountain plaza             |
| `hall`    | the single hall, the default                                     |

The four gardens are generated from your accounts and the desks they need, so
the campus grows with them. `hall` is the built-in layout and never changes.

- `clalt office --template <id>` starts with that template for this run. It
  writes nothing: `active.json` stays as it was.
- On the page you can switch templates while watching. That is a read
  (`GET /api/layout?template=<id>`), the page remembers the pick, and no file
  is written. The default template is chosen in the page's settings and saved
  for every run (the same as `clalt office layout use <id>`).
- `clalt office layout list` shows the templates next to your layouts.
  `active.json` may name a template or a saved layout. With nothing set the
  active one is `hall`.
- A generated layout is never stored. If a template cannot be generated the hall
  is shown and the reason goes to `office.log`.

## Layout formats

Version 1 is the hall (rooms and furniture on a grid) and keeps loading.
Version 2 is the garden: a site (island outline, grass, river, ponds, paths,
bridges, props, trees), pavilions with a type, a position and a rotation, and
overflow spots. A file holds the keys of its own version only, the other
version's keys are refused. Every number is finite and inside 200, pavilions
stay inside the island and do not overlap each other, the river or a pond, and
there is exactly one lobby. The pavilion sizes come from
`internal/office/layout/data/footprints.json`. A layout that names a template is
generated and cannot be saved. Editing version 2 layouts in build mode is not
there yet, build mode edits version 1 layouts.

## Build mode

Build mode opens from the page and lets you change the floor plan: move rooms
and furniture, then save. It needs no flag. The write routes are served in every
run and are protected by the guards below, not by a switch.

- A layout is one json file in `~/.config/clalt/office/layouts/<name>.json`.
  The active one is named in `~/.config/clalt/office/active.json`. With no valid
  active layout the default template (`hall`) is shown, and a bad file is logged to
  `office.log` and skipped, never a crash.
- The built-in templates live inside the binary and are read only. Saving while
  one is active writes a copy named `<name>-custom` and switches to it.
- Writes need the run's token, the header `X-Clalt-Edit: 1`, an `Origin` that is
  this server, a json content type, and a body under 512 KB. A page of another
  site cannot meet that.

From the command line, which needs no running office:

    clalt office layout list
    clalt office layout use <name>
    clalt office layout export <name> [file] [--force]
    clalt office layout import <file> [--name n] [--force]
    clalt office layout reset

`use` takes a template id too. `reset` points the active layout back at the
default one (`hall`) and deletes nothing.
`export` writes json to stdout, or to a file that must not exist yet unless you
pass `--force`.

Sharing layouts is safe because every file is treated as untrusted. Import,
load and save decode strictly (unknown keys are refused), check the size, the
colours, every number against the floor and the catalog of furniture, and
refuse anything that looks like a url. A layout holds no code, no links and no
paths, and the page shows its text as text. An import that fails says why and
changes nothing.

## Privacy

- The page is served from your machine only. The server listens on `127.0.0.1`
  and nothing else, with a random port unless you set one (the background
  office uses 47321 or the next free port). It needs no network access (only
  the opt-in remote login fetches keys, see [office-remote.md](office-remote.md)) and the page loads
  nothing from the internet: the 3D library, the models and the page are inside the clalt binary.
- Every run makes a new random token (the background office keeps it in
  `run.json`, readable by you only). The printed link carries it, the first
  load turns it into a cookie that only this browser sends to this server, and
  every request without it is refused. Other websites in your browser cannot
  read the page or the data, and a request that names another host (a DNS
  rebinding attempt) is refused. Anyone who has the link while the office runs
  can look, so do not paste it in a chat.
- The page watches, and saves only layouts, the default template and the idle
  choices (`/api/idle`, see [office-idle.md](office-idle.md)). Reads are GET requests for the embedded page files, the snapshot,
  the active layout and the activity log of one session (see below). Three
  more routes (`PUT /api/layout`, `GET /api/layouts`, `POST /api/layout/use`)
  change or list layouts, behind the guards (see [office-versions.md](office-versions.md)).
- What the snapshot holds: the account ids and labels, the 5h and 7d limit
  percentages and reset times, and per session its handle, account, the name of
  its project folder (not the path), the model, its state, a short reason when it
  waits for you, when it started, and its subagents (type, a short description,
  state, times). It holds no prompts, no transcript text (the activity panel is
  the one place that does, on request), no emails, no tokens,
  no environment and no full paths. Text is cleaned of control characters. An
  account label that looks like an email address is cut at the at sign, so the
  page never shows an address.
- The office itself writes nothing to disk except errors, one line each, in
  `~/.config/clalt/logs/office.log`, and layouts you save in build mode. The token is never logged.

## Activity panel

Opening a session, a team desk or an intern shows what that session did:
prompts and replies as text, tool calls as compact rows (tool, files, how long,
whether it worked), and subagent runs as groups. It comes from
`GET /api/activity?handle=<session handle>[&agent=<subagent id>][&before=<cursor>][&after=<cursor>][&limit=<n>]`,
which has the same guards as the snapshot (the token and the Host check).

- clalt reads the session's transcript when the panel asks and keeps nothing:
  no copy, no index, no file written. A reply is reused for one second so a
  page that polls costs one read. The only thing that persists is claude's own
  transcript, which clalt never changes.
- It reads backwards from the end of the transcript, so the newest entries are
  cheap, and scans at most 64 MB per request. `next` in the reply is the cursor
  for older entries and is empty when there are none. `limit` is 100 by default
  and at most 300. `newest` in the reply of the newest page is the cursor to
  pass back as `after` to get only what came since, newest first; a poll that
  finds more than a page follows `next` with `before` and the same `after`. A
  page can run a few entries over it to keep a tool call
  with its result.
- Text is cleaned of control and format characters (newlines and tabs stay),
  each text is cut at 20000 characters with a `[cut]` marker, and a reply holds
  about 2 MB at most. Lines that cannot be read are skipped and counted in
  `skipped`.
- The transcript is found the way usage and `clalt resume` find it:
  the session's claude id, then the project folder named after its working
  folder, with a scan of the projects folders as fallback. A session without
  a transcript yet, or an unknown handle, answers 404 with a short reason.
- A subagent run is the file `agent-<id>.jsonl` in the session's `subagents`
  folder, which is where claude writes them now. For older transcripts that
  kept sidechain lines in the main file, those lines of that agent are used.
  The main view shows a run as a start and an end entry. A run started in the
  background has a start entry only, since its end is not in the transcript.
- Unlike the snapshot, this does carry transcript text (your prompts, replies,
  tool inputs and results), to the page of this browser only.

## Today and History

The Today tab shows the day of every account that did something: sessions,
active time, tokens, cost, tools, files and failures. Per account it also
shows the 5h limit as the day's peak and the latest reading, with a meter. Copy
as text carries that line too.

The History tab charts, per account and for 7, 30 or 90 days, the highest 5h
percent and the highest 7d percent read on each local day (0 to 100, with a
line at 90 percent), then the tokens per day and the busiest 5h window in
tokens. A day with no reading is a gap, not 0. The percents come from the
readings log of the feed, which began when you installed this version, so older
days show tokens only and the page says when the readings began. See
[feed.md](feed.md#the-readings-log) for what the log holds and how long.

## Where the data comes from

The same places as the dashboard: the accounts of clalt, the registry of
running `clalt run` sessions, and the feed that claude's hooks fill, which
includes plain `claude` sessions when [the feed is installed for
it](feed.md). Without the feed, sessions show as starting and limits as
unknown. Subagents come from the feed's subagent hooks too. An existing feed
install needs `clalt feed install` run again to add them, see
[feed.md](feed.md).

## Credits

- [three.js](https://threejs.org) r170, MIT license.
- [Kenney Furniture Kit](https://kenney.nl/assets/furniture-kit) and
  [Kenney Mini Characters](https://kenney.nl/assets/mini-characters), CC0.

The same list is on the page, under "credits" at the bottom right.
