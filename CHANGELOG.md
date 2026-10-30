# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

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

- PR review cards no longer overlap: long titles wrap inside the card (up to
  three lines, full title on hover) and the counts stay inside (#76).
- The Reviews count sits inside the Reviews button instead of the corner of
  the button row.

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
