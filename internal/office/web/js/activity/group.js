// folds subagent runs into groups. pure. input is chronological (oldest first).
// a group sits where its run first shows up. a run whose start is on an older page still groups,
// it just has no start marker. an item is {entry} or {group: {agent, start, end, entries, state}}.
const isMarker = (e) => e.kind === 'subagent-start' || e.kind === 'subagent-end';

export function groupEntries(chrono, { flat = false } = {}) {
  const items = [];
  const groups = new Map();
  const groupOf = (agent) => {
    let g = groups.get(agent);
    if (!g) {
      g = { agent, start: null, end: null, entries: [], state: 'running' };
      groups.set(agent, g);
      items.push({ group: g });
    }
    return g;
  };
  for (const e of chrono) {
    if (flat) { if (!isMarker(e)) items.push({ entry: e }); continue; }
    if (!e.agent) { items.push({ entry: e }); continue; }
    const g = groupOf(e.agent);
    if (e.kind === 'subagent-start') g.start = e;
    else if (e.kind === 'subagent-end') { g.end = e; g.state = 'done'; } else g.entries.push(e);
  }
  return items;
}
