# The office

`clalt office` opens a live 3D office in your browser. Every session of every
account is a character at a desk, every subagent is a character too. It is a
viewer: nothing on the page starts, stops or changes a session.

    clalt office [--port N] [--no-open] [--edit] [--template <id>]

It prints a link, opens it in your default browser, and runs until you press
Ctrl-C. `--no-open` only prints the link. `--port N` picks the port, by default
a free one.

## How it looks

- A glass cabin per session, grouped in a wing per account. The wing sign shows
  the account label and its 5h and 7d limit bars. A bar with a question mark is
  a window the feed has no reading of yet, which is not 0 percent.
- The sign over a cabin and the tag over a character show the project folder of
  the session. Subagents show their agent type.
- Subagents sit at desks by kind: Explore in Research, Plan in the Meeting room,
  reviewers and test agents in the Review room, everything else in the Open
  workspace. When a room is full the rest sit in the Open workspace.
- A busy session types, with a spinning ring over its head. An idle one gets up
  now and then and walks to the cafe or the play area, chats, plays ping-pong or
  the arcade, and comes back. A session that needs you stands at its cabin door
  and waves with a bubble over its head.
- A new session or subagent walks in through the lobby. A finished subagent
  hands a paper to its main agent and leaves. An ended session walks out.
- With nothing running the office is empty and says so.

## Controls

- Drag to orbit the whole way round, right drag (or shift and drag) to pan, wheel
  to zoom, two fingers to pinch and pan on a touch screen.
- Top view, Front view and Overview buttons (Esc goes back to the overview too).
- Hover a character for a card with the handle, project, model, state and
  account label. Click a room for the number of agents in it.
- Team lines draws dotted lines from each subagent to its main agent. Tags shows
  every tag, not only the ones of the main agents.

## Templates

The office starts as a garden campus: pavilions on a lawn with paths, water and
trees. There are five templates:

| id        | what it is                                                       |
| --------- | ---------------------------------------------------------------- |
| `rows`    | cabins in rows along the back, the default                       |
| `village` | pavilions on a ring around a green with a pond and the cafe      |
| `river`   | cabins along the west bank of a river                            |
| `street`  | one winding street from the gate to a fountain plaza             |
| `hall`    | the old single hall, kept as it was                              |

The four gardens are generated from your accounts and the desks they need, so
the campus grows with them. `hall` is the built-in layout and never changes.

- `clalt office --template <id>` starts with that template for this run. It
  writes nothing: `active.json` stays as it was.
- On the page you can switch templates while watching. That is a read
  (`GET /api/layout?template=<id>`), the page remembers the pick, and no file
  is written. Making a template the saved default is a write like any other:
  `clalt office layout use <id>` or, with `--edit`, the page.
- `clalt office layout list` shows the templates next to your layouts.
  `active.json` may name a template or a saved layout. With nothing set the
  active one is `rows`.
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

`clalt office --edit` lets you change the floor plan from the page: move rooms
and furniture, then save. Without `--edit` the server has no write routes at
all: with the run's token they answer 404, without it 401 like every route.

- A layout is one json file in `~/.config/clalt/office/layouts/<name>.json`.
  The active one is named in `~/.config/clalt/office/active.json`. With no valid
  active layout the default template (`rows`) is shown, and a bad file is logged to
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
default one (`rows`) and deletes nothing.
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
  and nothing else, with a random port unless you set one. It needs no network
  access and the page loads nothing from the internet: the 3D library, the
  models and the page are inside the clalt binary.
- Every run makes a new random token. The printed link carries it, the first
  load turns it into a cookie that only this browser sends to this server, and
  every request without it is refused. Other websites in your browser cannot
  read the page or the data, and a request that names another host (a DNS
  rebinding attempt) is refused. Anyone who has the link while the office runs
  can look, so do not paste it in a chat.
- The page only reads, unless you start it with `--edit`. Only GET requests
  are answered, and only the embedded page files, the snapshot and the active
  layout. With `--edit` three more routes accept layout changes, see build mode.
- What the snapshot holds: the account ids and labels, the 5h and 7d limit
  percentages and reset times, and per session its handle, account, the name of
  its project folder (not the path), the model, its state, a short reason when it
  waits for you, when it started, and its subagents (type, a short description,
  state, times). It holds no prompts, no transcript text, no emails, no tokens,
  no environment and no full paths. Text is cleaned of control characters. An
  account label that looks like an email address is cut at the at sign, so the
  page never shows an address.
- Nothing is written to disk except errors, one line each, in
  `~/.config/clalt/logs/office.log`, and layouts you save in build mode. The token is never logged.

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
