# Idle choices in the office

What an agent does while it has nothing to do is a setting. Open Settings, find
"When idle" and press "Choose what idle agents do", or use the "When idle" menu
at the top of the activity panel of any agent. The choices:

- Stay at desk: it never gets up. An agent already on a break finishes it and
  walks back.
- Take a break: the cafe and play area, as before.
- Hide until it works again: the character, its desk setup, its tag and its ring
  are not shown and cannot be clicked. It comes back when it is busy or needs
  you. Hidden agents are idle, so they add nothing to the number on a room sign.
- Rest in a room (hall only): it walks to a free place in that room, a bean bag
  where there is one, and stays until it has work. If the room has no free place
  it takes a break instead. A room that is gone from the layout also means a
  break. The garden has no room choices.

The most specific rule wins. A team member uses its own rule, else the team
members default. An intern (a later run of a team type) uses its team rule, then
the rule of its subagent type, then its account, then everyone. A plain subagent
uses its type, then its account, then everyone. A session uses its account, then
everyone. The out-of-the-box answer is: sessions and subagents take a break, team
members stay at their desk. A team member set to take a break needs a real desk
seat to leave from; one without it stays at the desk.

The quick menu always saves: a change edits the matching rule (team member,
account, subagent type) for everyone like it, and shows what it saved for. Both
places read the same rules, saved by the server in `/api/idle`. An older server
without that route keeps today's behaviour and the controls stay hidden.

## The route

`GET /api/idle` and `POST /api/idle` read and save where idle agents go. The
rules live in `idle.json` next to the layouts, mode 0600, written atomically.

    {"default":"break","teamDefault":"desk","accounts":{},"teams":{},"subtypes":{}}

- Each choice is `desk`, `break`, `hide` or `room:<id>` (a room id of a layout:
  1 to 32 characters of a-z, 0-9 and `-`). The room is not checked against the
  active layout, the page falls back when it is missing.
- `default` and `teamDefault` are required. `accounts` is keyed by account id,
  `teams` and `subtypes` by 1 to 64 printable ASCII characters. At most 200
  entries per map.
- A missing, unreadable or invalid file answers the defaults above (invalid
  content is logged, never a 500).
- The POST has the guards of the other writes: token, edit header, origin,
  JSON content type and body size. It answers 200 with the saved rules, or 400
  with a one-line message.
