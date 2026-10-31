# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Rooms that meet on the hall deck (the Review and Meeting rooms) are
  separated by a glass wall with a doorway at the front; agents walk
  through the doorway.
- The account card in the hall says where its sessions are, for example
  "1 working, 2 on break" (or "1 needs you"), above the 5h and 7d bars.
- The number on a room sign counts only the agents working in it, not the
  ones sitting idle at a desk.
- Every session and subagent stands on a small flat ring in its account
  colour, in the garden and the hall. Team members have none.
- In the hall, the sessions of one account sit side by side in the shared
  cabins: each account gets one block of cabins, accounts in name order.
- The office opens in the hall by default (fresh installs, and after the
  active saved layout is deleted or reset). The garden templates stay one
  click away in the settings.
- Agents at their desks act out the work they are doing: leaning in to read,
  typing fast, leaning back while a command runs, a hand to the forehead when
  it fails, arms up when it passes, browsing for the web. The screen glows and
  a bubble shows the same thing. The snapshot gains an additive `doing` field
  per session and subagent, built from the transcript tail.

### Changed

- The Settings dialog uses the field notebook look: card-style options,
  inked buttons with hover, pressed, focus and disabled states, amber main
  actions (#76).
- Settings opens as a two-page notebook spread: office templates as picture
  cards with Build mode on the left, PR reviews and start at login on the
  right. A View section holds labels, team lines, every tag, auto-spin and
  the walking camera (keys L, P and G still work).
- The bottom of the office keeps three centered buttons: Today, Reviews and
  Settings.

### Fixed

- The Reviews badge counts only reviews still open, not merged or closed
  pull requests (those are in the History tab).
- PR review cards no longer overlap: long titles wrap inside the card (up to
  three lines, full title on hover) and the counts stay inside (#76).
- The Reviews count sits inside the Reviews button instead of the corner of
  the button row.
- Account cards in the garden office no longer slide under the status pill or
  the bottom buttons: label fitting now treats both as fixed obstacles.

## [0.3.0] - 2026-10-28

### Added

- `clalt office on|off|open|status [--link]` runs the office in the
  background without a terminal (#69); `clalt office autostart on|off|status`
  and a Settings checkbox start it at login on macOS (launchd). The run token
  stays in a 0600 run file and the browser cookie only.
- `clalt mcp`, a read-only stdio MCP server, and a Claude Code plugin (#65):
  tools limits, best_account, sessions and today. Install with
  `/plugin marketplace add dhitalkamal/clalt` and `/plugin install clalt@clalt`.
- Percent history (#64): the feed logs each limit reading per account (numbers
  only, 90 days) and the office History tab charts the 5h and 7d peak percent
  per day with a 90 percent line. Today shows the 5h peak and the latest
  reading.
- The hall shows only the hall (#72): no outdoor scenery, a plain background,
  and walls near the camera fade when you orbit low. The camera stays on the
  area in every template and never goes below the ground.

### Changed

- The office detail panels (activity drawer, hover cards, Today and History,
  Reviews) use a field notebook style (#70, #73).
- The office draws on demand (#71): full speed only while you move the camera,
  slower while only agents move, about 2 frames a second when nothing moves;
  static shadows are baked once; pixel density is capped. Shift+F shows the
  frame rate. docs/office.md has a Battery note.
- The camera bar drops the top, plan, iso and hero presets (keys 6 to 9, #68):
  the sky camera orbits freely, and Home, Esc and Overview go to one overview.

### Fixed

- Working agents and interns get desks before idle team members, and idle
  members rest in the cafe when desks run out (#67).
- One Esc closes the activity drawer and the pinned agent card; the Reviews and
  Today panels no longer cover the controls row (#73).
- Two tests that failed under heavy load (#66).

## Older releases

- [0.2.0] - 2026-10-28: see docs/changelog/0.2.0.md
- [0.1.0] - 2026-10-22: see docs/changelog/0.1.0.md
