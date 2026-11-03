# Agents that work like people

Approved 2026-10-31 from the throwaway mockup humans.html. The office layout
stays as it is; only what the agents do changes. Every action comes from a
real event in the session, never from a random timer.

## Behaviours and their triggers

| Event (source) | What the agent does |
|---|---|
| Read, Grep, Glob (activity tool) | leans in, reads the screen |
| Edit, Write, NotebookEdit | types fast |
| Bash running | leans back, waits |
| Bash done, ok false | hand to the forehead, a short moment |
| Bash done, ok true after a wait | arms up, a short moment |
| WebSearch, WebFetch | browses |
| needs you (snapshot state and reason) | stands, raises a hand, waves until answered, then sits |
| subagent starts (snapshot subagents) | main agent walks to the subagent's desk, briefs it, walks back |
| subagent ends | subagent walks to its main agent, reports, then leaves |
| idle (snapshot state) | coffee, cooler chat with another idle agent, as the break code does today |
| session appears or ends (snapshot) | arrives through the entrance, leaves through it |

The screen of an agent's desk glows in a colour for its current work, and the
bubble over the head shows the same thing.

## Data

The snapshot gains one field per session and per subagent:

    "doing": {"kind": "read|edit|run|web|other", "tool": "Bash",
              "state": "running|ok|fail", "at": "2026-10-31T10:00:00Z"}

- Built in internal/office from the transcript tail with the existing
  activity parser: the newest tool call, whether its result has arrived and
  whether it was ok. A tool name maps to a kind by a fixed table.
- No file names, commands or text leave the parser in this field.
- Missing or old (older than 2 minutes) means no current work: the agent
  falls back to today's busy pose.
- Read cost: the tail only (the last few hundred kB), at most once per
  snapshot poll per session, cached by file size and mtime.

## Delivery, one PR each, test first

1. feat/agent-work-poses: the `doing` field (Go, table tests for the tool
   to kind map, running vs done, ok vs fail, staleness, caching), and the
   seated work poses, screen glow and bubbles in the page (pure mapping in
   js/behave/, node tests).
2. feat/agent-asks-for-help: standing and waving while needs you, sitting
   back down when answered.
3. feat/agent-teamwork: brief and report walks for subagents, arriving and
   leaving through the entrance.

Reduced motion: no walks or waves, the pose and bubble change in place.
Checks: make check, node tests for every pure module, and a browser pass
with a replay of fake transcript events.
