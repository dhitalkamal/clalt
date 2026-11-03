# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.4.2] - 2026-11-03

### Fixed

- Windows: a state file can be replaced while another clalt process is
  reading it (reads open it with FILE_SHARE_DELETE and the save uses a
  posix-style rename), so a busy machine no longer fails a save with "Access
  is denied".
- Windows: PR reviews find gh and claude under your home (.local\bin,
  go\bin) as .exe, .cmd or .bat, and take a path written with either slash.
- The test suite passes on Windows, and the Windows CI job fails the build
  again.

## [0.4.1] - 2026-11-03

First public release.

### Added

- SECURITY.md with private vulnerability reporting, CONTRIBUTING.md, issue
  and pull request templates.
- The README points to the release archives and how to verify them, and
  shows how to install the Claude Code plugin.

### Changed

- The demo dashboard and the tests use made-up names (robin, casey, morgan,
  jordan, harriet).
- The full command list moved from the README to docs/commands.md.
- The README and docs/move.md say how to use clalt within Anthropic's terms:
  your own accounts only, kept apart, not combined to get more usage.
- The plugin manifest reports version 0.4.1 and links docs on main.

### Known issues

- Windows stays best effort. Its CI run has known test failures (tests that
  expect unix file modes and paths, and `feed install`, which is refused on
  Windows), and saving the state file can fail while another process reads it.
  The Windows CI job no longer fails the build.

## [0.4.0] - 2026-11-03

### Added

- Open the office from your phone or another computer through a Cloudflare
  Tunnel: with an access.json, the office trusts Cloudflare Access as the
  login on that one public hostname and checks every signed request itself
  (#93, see docs/office-remote.md).
- PR reviews keep their history: the panel shows reviews of open pull
  requests, and reviews of merged or closed ones move to a History tab (#77).
- Big pull requests are reviewed in chunks instead of being skipped or cut;
  a chunk that fails marks the review as partial, never as clean (#78).
- A review that fails (a network blip, a failed clone) is retried at the
  same commit after 5m, 15m, 1h and 3h, up to 5 attempts, and the panel says
  when the next try is (#79, #80).
- Delete saved layouts from Settings or the Build mode Layouts menu (#87).
- When a subagent starts, its main agent gets up, walks to the new helper's
  desk, briefs it for a moment and walks back. With reduced motion agents
  that need you raise a hand in place instead of walking, and a seat with no
  room to stand raises a hand from the chair.
- Idle choices: pick what an idle agent does (stay at its desk, take a break,
  hide until it works again, or rest in a room of the hall) per account, team
  member, subagent type or for everyone, from Settings ("When idle") or the
  activity panel of an agent. Team members can now take breaks when asked.
  Saved in idle.json next to the layouts (`GET` and `POST /api/idle`).
- The Review and Meeting rooms upstairs in the hall are separate rooms: a
  solid wall stands between them and a corridor runs in front from the
  stairs, with a glass front and a door for each room, so nobody walks
  through one room to reach the other. The meeting room has two desks;
  extra planners sit in the workspace.
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

- The background office started at login can run reviews again: it gets the
  PATH of your shell and finds gh and claude in the usual places, and
  `clalt office status` warns when it cannot. Run `clalt office autostart on`
  once to rewrite the login item (#81).
- 19 camera fixes: the sky camera no longer flies off or slides at the zoom
  limit, modes no longer drift or fight each other, keys held while a dialog
  opens or the tab hides are released, and touch pinch works (#88).
- The office no longer grows slower each time you switch layouts.
- Agents in the hall no longer slide through desks and cabins when their
  seat changes: they get up and walk over the floor to the new seat, leaving
  a cabin through its door. A walk that starts inside furniture now heads for
  the nearest open floor first instead of cutting straight across.
- Two office tabs no longer overwrite each other: idle choices merge the
  change of each tab, and the layout editor asks before replacing a layout
  another tab saved ("Load theirs" or "Keep mine").
- The Reviews badge counts only reviews still open, not merged or closed
  pull requests (those are in the History tab).
- PR review cards no longer overlap: long titles wrap inside the card (up to
  three lines, full title on hover) and the counts stay inside (#76).
- The Reviews count sits inside the Reviews button instead of the corner of
  the button row.
- Account cards in the garden office no longer slide under the status pill or
  the bottom buttons: label fitting now treats both as fixed obstacles.
- PR reviews: repository names that differ only in case are one entry, and
  the add box takes github.com urls, pull request urls, a trailing slash or
  .git.
- Deleting the active layout reloads the fallback layout in an open editor, so
  Save cannot bring the deleted layout back.
- The editor refuses an empty layout title, like it already did for odd spacing.
- The activity panel menu says who a change applies to before you save, and
  "saved for ..." only after a save.
- Account cards say "resting" for agents in a room chosen in the idle settings,
  "on break" only for cafe and play area breaks.
- A stale tab says it lost its access in plain words and shows no raw
  "unauthorized" when a save fails.
- Long layout titles are cut with an ellipsis in the editor Layouts menu, and
  on short wide windows the account cards stay below the status pill.
- office.log logs a skipped (invalid) layout file once, not on every poll.

## Older releases

- [0.3.0] - 2026-10-28: see docs/changelog/0.3.0.md
- [0.2.0] - 2026-10-28: see docs/changelog/0.2.0.md
- [0.1.0] - 2026-10-22: see docs/changelog/0.1.0.md
