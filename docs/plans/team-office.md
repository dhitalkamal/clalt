# team office

the office reads like a real team: a busy agent works at its desk and does
nothing else, every session has its own character, and every agent type the
user has (built-in and custom) has a permanent character and desk on the map.

## decisions (agreed with the user)

- working means working: a busy session or running agent stays at its desk and
  types. only idle ones take breaks. needs-you stands at the desk and waves.
- one dedicated character per session: the look comes from a hash of the
  session handle, the seat stays the same for the whole session.
- the team is always on the map: every agent type has a named desk with a
  nameplate in a team area. not running: dimmed, resting at the desk. running:
  lights up and works.
- interns: when an agent type already runs and another run of the same type
  starts, the extra run is an intern: a smaller character in the agent's colour
  with a lanyard, on a stool beside the agent's desk (3 stools per desk, more
  shown as a +N badge). when an intern's run ends it walks to the session that
  started it, hands over the paper (existing handoff) and leaves by the lobby.
  the agent itself is never an intern: the first run is always the agent.
- which agents: built-in types plus custom agents from ~/.claude/agents and the
  .claude/agents folder of every project with a running session, plus any type
  seen in the feed that is not in the list (plugin agents, for example).
- privacy: only the name and the one-line description from an agent file's
  front matter are read, never the body.

## contract

snapshot (GET /api/snapshot) gains `team`, version stays 1 (additive):

```
"team": [
  {"id": "explore",            // [a-z0-9-:]{1,64}, lowercase of the type
   "name": "Explore",          // display, <= 40 chars, cleaned
   "desc": "fast codebase search", // <= 120 chars, cleaned, may be ""
   "source": "builtin|user|project|seen",
   "busy": 2,                  // running subagents of this type right now
   "color": "#4f8edc"}         // stable colour from the id hash
]
```

- order: builtin (general-purpose, explore, plan, then reviewers seen), then
  user, then project, then seen, each by name. at most 24 entries; the rest are
  dropped from the team (their runs still show as generic agents).
- built-in list: general-purpose, Explore, Plan. a reviewer type is any type
  matching review|test|hunter|analy (the page's existing rule).
- type matching is case-insensitive on the subagent's `type` field; the
  plugin prefix stays part of the id (pr-review-toolkit:code-reviewer).
- agent files: *.md only (skip backups like *.md.pre-*), front matter
  `name:` and `description:` only, file size cap 64 KB, at most 64 files per
  folder, symlinks not followed out of the folder, read errors skip the file.
- the team source is cached and re-read at most every 30 s.

layout v2: new pavilion type `team` with `cols` and `rows` like workspace
(footprint col 2.1, row 3.5, pad 0.2, desks cols*rows*2 like workspace), and the
pavilion gains an optional `members: ["explore", ...]` list (ids, in desk
order, <= 64). generators take `TeamIDs []string` in their input and add one
team pavilion per 8 members next to the workspace, so templates grow with the
team. the hall (v1) has no team pavilion: team members take the hall's
workspace and research desks in order, extra members stand in the lobby.

routing (page, pure): the roster gains
- `{key: "team:<id>", kind: "team", team: id, state: busy|idle}` for every
  team entry, always present;
- interns `{key: <subagent id>, kind: "intern", team: id, owner: <session handle>}`
  for runs beyond the first of a type;
- the first run of a type maps onto its team member (no extra character).
planSeats gives each team member its desk in the team pavilion (by members
order), interns the stools beside it, sessions their cabin desk as today.
breaks are only planned for agents whose state is idle.

## work split (one pr, three parallel streams)

1. go: team discovery (internal/office/team.go and tests), snapshot field,
   layout v2 `team` pavilion type and `members`, footprint, validator rules,
   generators take team ids and place team pavilions, garden feed passes them.
2. web routing and behaviour: roster with team, interns and first-run mapping,
   planSeats for team desks and intern stools (v2 and hall), busy never breaks,
   stable session looks, hover card text for team members and interns.
3. web render: team pavilion builder (desks with nameplates, intern stools),
   dimmed idle look and lit busy look, intern character (smaller, lanyard,
   agent colour), +N badge.
