// turns a reply's entries (newest first) into the rows the panel draws, oldest first. pure.
// row types: text (prompt, reply, note), tool, result, group (a subagent run with children).
import { entryKey } from './merge.js';
import { groupEntries } from './group.js';
import { truncateText, snippet, formatDuration, clock, fileSummary } from './text.js';

const ROLES = { prompt: 'prompt', reply: 'reply' };
const LABELS = { prompt: 'You', reply: 'Claude', note: 'Note' };
const status = (e) => (e.ok === false ? 'failed' : 'ok');

export function buildRows(entries, { flat = false, now = new Date() } = {}) {
  const chrono = [...entries].reverse();
  const seen = new Map();
  // the n-th identical entry counted from the oldest, so ids hold when newer entries arrive
  const idOf = (e) => {
    const k = entryKey(e);
    const n = seen.get(k) || 0;
    seen.set(k, n + 1);
    return k + '#' + n;
  };
  const base = (e) => ({ id: idOf(e), time: clock(e.t, now), iso: e.t || '' });

  const rowOf = (e) => {
    if (e.kind === 'tool') {
      return { ...base(e), type: 'tool', tool: e.tool || 'tool', files: fileSummary(e.files), detail: (e.files || []).includes(snippet(e.text, 400)) ? '' : snippet(e.text, 120), duration: formatDuration(e.durationMs), status: status(e) };
    }
    if (e.kind === 'result') {
      const text = typeof e.text === 'string' ? e.text : '';
      const one = snippet(text, 140);
      return { ...base(e), type: 'result', snippet: one, full: text, cut: text.trim() !== one, status: status(e) };
    }
    const role = ROLES[e.kind] || 'note';
    const text = typeof e.text === 'string' ? e.text : '';
    const t = truncateText(text);
    return { ...base(e), type: 'text', role, label: LABELS[role], text: t.text, full: text, cut: t.cut, hidden: t.hidden };
  };

  return groupEntries(chrono, { flat }).map((item) => {
    if (item.entry) return rowOf(item.entry);
    const g = item.group;
    const first = g.start || g.entries[0] || g.end;
    const ms = g.end && g.end.durationMs > 0 ? g.end.durationMs : g.start && g.end ? Date.parse(g.end.t) - Date.parse(g.start.t) : 0;
    const children = g.entries.map(rowOf);
    return {
      id: 'g:' + g.agent, type: 'group', agent: g.agent, state: g.state, status: g.end ? status(g.end) : 'ok',
      title: (g.start && snippet(g.start.text, 100)) || 'subagent ' + g.agent.slice(0, 8),
      time: clock(first.t, now), iso: first.t || '', duration: formatDuration(ms), count: children.length, children,
    };
  });
}
