# Office build mode

Shared plan for the Go side (internal/office/layout, server routes, CLI) and
the page side (internal/office/web). Layout json v1 contract first, then the
security design and the HTTP shapes the page relies on.

## Layout json v1

file: ~/.config/clalt/office/layouts/<name>.json, active one named in
~/.config/clalt/office/active.json {"name": "<name>"}
name: [a-z0-9-]{1,40}

    {
      "v": 1,
      "name": "default",
      "title": "Default office",              // display, <= 60 chars, cleaned
      "grid": 0.5,                             // world units per cell, fixed 0.5 in v1
      "size": {"w": 46, "d": 18},              // cells, 8..120 each
      "walls": {"color": "#e9e4dc"},
      "floorColor": "#d9b88f",
      "door": {"side": "left|right|back|front", "at": 6},      // cell offset along that side
      "deck": {"x": 30, "z": 0, "w": 16, "d": 8} | null,       // upper level 1 area in cells; null = no upper floor
      "stairs": {"x": 44, "z": 8, "dir": "n|s|e|w"} | null,    // required when deck is set
      "rooms": [
        {"id": "r1", "type": "lobby|cabins|research|workspace|review|meeting|cafe|play",
         "level": 0, "x": 0, "z": 0, "w": 6, "d": 6,          // cells, inside size (and inside deck when level 1)
         "label": "Lobby", "floor": "#hex or omitted", "account": "<account id or omitted>"}  // account only for cabins
      ],
      "furniture": [
        {"id": "f1", "item": "<catalog id>", "level": 0, "x": 3, "z": 2, "rot": 0|90|180|270, "room": "r1" or omitted}
      ]
    }

rules (go validates, page enforces while editing):

- ids: [a-z0-9-]{1,32}, unique per list; at most 64 rooms and 1500 furniture.
- exactly one lobby room on level 0; other types optional, any number.
- rooms on the same level do not overlap; furniture inside size (and deck for
  level 1); furniture must not overlap another furniture footprint (catalog
  footprint, rotated).
- unknown keys rejected; strings cleaned (no control or format chars); colors
  #rrggbb only; no urls anywhere.
- file size cap 512 KB.
- catalog ids: the GLB names already embedded in internal/office/web/assets plus
  any procedural items the page draws today (desk, chair, monitor, lamp,
  plant-*, bookcase, sofa, rug, coffee-machine, ping-pong, arcade, bean-bag,
  whiteboard, reception, water-cooler, printer, ...). the page builder writes
  the authoritative list to internal/office/layout/data/catalog.json:
  [{"id","footprint":{"w","d"},"levels":[0,1],"tags":[...]}]; the go builder
  embeds and validates against it.
- agent routing by room type: cabins -> main agents (one cabin desk per
  session; with "account" set only that account's sessions), research ->
  Explore, review -> reviewers, meeting -> Plan, workspace -> general-purpose
  and unknown types, cafe and play -> idle breaks, lobby -> arrivals. seats come
  from furniture items tagged "seat" inside the room; when a room is full or
  missing, agents use any workspace seat, then stand in the lobby.

templates (page builder authors them, go embeds):
internal/office/layout/data/templates/default.json (the current office, same look),
compact.json, loft.json. scope cut for the first PR: only default.json ships
(compact and loft come later) and the editor edits level 0 only, but the
validator still accepts and checks the deck, the stairs and level 1 items.

## Geometry conventions (Go validator)

- room x,z,w,d are cells, x,z is the minimum corner. rooms are half open boxes
  [x, x+w) by [z, z+d).
- furniture x,z is the minimum corner of the footprint box after rotation: at
  rot 90 and 270 the catalog footprint w and d swap. the box must be inside
  size (level 0) or inside the deck (level 1) and must not overlap another
  furniture box on the same level.
- a furniture item needs its level in the catalog item's levels (empty means
  both). "room" must name an existing room on the same level.
- door.at is a cell offset along the side: 0..d-1 for left and right, 0..w-1
  for back and front. stairs.x,z is a cell inside size.
- stairs must be null when deck is null, and set when deck is set. x,z is the
  top most cell of a flight 2 cells wide and 6 long, dir is the way up (n is
  toward smaller z, e toward larger x), the flight runs the other way from the
  top and a 2x2 landing sits past the top. flight and landing must be inside
  size.
- catalog items tagged "floor" (rugs, mats) are exempt from the furniture
  overlap check.
- the lobby is exactly one room, on level 0.
- account is [A-Za-z0-9._-]{1,64} and only on cabins rooms.
- strings: title <= 60, room label <= 40, cleaned of control and format
  characters. anything that looks like a url (://, a leading //, www.,
  javascript:, data:) is refused in any string. colours are #rrggbb.

## Security design

Layout files are untrusted input: users share them, so import and load treat a
file like a request body.

- strict decoding: unknown keys refused, 512 KB cap, one json value only.
- validated against the embedded catalog on load, save, import and use. a file
  that fails is never served: the active layout falls back to the default one
  and the failure is logged (never a crash).
- the layout holds no code and no urls, and the page sets every string as text.
- files live in ~/.config/clalt/office/layouts, names are [a-z0-9-]{1,40} so a
  name can never be a path. built-in templates are read only (embedded) and
  cannot be overwritten or deleted.
- the server is watch only unless started with `clalt office --edit`. the token
  is checked first (401 without it, as for every route), then without --edit
  PUT /api/layout, GET /api/layouts and POST /api/layout/use answer 404, so
  there is no write surface at all.
- numbers are range checked (0..120 for coordinates and sizes) before any sum,
  so huge values cannot wrap around. names that are Windows device names (con,
  prn, aux, nul, com1-9, lpt1-9) are refused.
- with --edit a write needs all of: the token (cookie or header), the header
  X-Clalt-Edit: 1, an Origin (or Referer) equal to the server's own origin
  (http://127.0.0.1:PORT or http://localhost:PORT), Content-Type
  application/json, and a body under the cap. A cross site page cannot set the
  custom header without a preflight, and no CORS header is ever sent.
- the Host check, the CSP and the token rules of the watch only server stay.

## HTTP shapes (page builder)

All routes need the token like /api/snapshot. /api/snapshot is unchanged.

GET /api/layout (always available)

    {"edit": false, "layout": { ...layout v1... }}

edit says whether the server was started with --edit. layout is the active
layout (the default one when nothing valid is active).

Only with --edit (otherwise 404 after the token check, 401 without a token). Writes need X-Clalt-Edit: 1, a matching
Origin and Content-Type: application/json.

PUT /api/layout, body = layout json. Validated and saved under the active
layout's name (the body name is ignored). When the active layout is a built-in
template the copy is saved as "<name>-custom" and made active. Answers 200 with
{"edit": true, "layout": {...saved layout, with its final name...}}, or 400
with a plain text reason.

GET /api/layouts

    {"active": "default", "layouts": [{"name": "default", "title": "Default office", "builtin": true}]}

POST /api/layout/use, body {"name": "loft"}. Makes it active, answers like
PUT. 404 for an unknown name, 400 for a stored file that no longer validates.

Errors are text/plain, one line, never a path.

## CLI

    clalt office [--port N] [--no-open] [--edit]
    clalt office layout list
    clalt office layout use <name>
    clalt office layout export <name> [file] [--force]
    clalt office layout import <file> [--name n]
    clalt office layout reset

reset points the active layout back at the default one and never deletes a
file. import validates and refuses with the reason.
