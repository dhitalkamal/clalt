// live problems list and the per room seat summary
import { h } from './dom.js';

export function createIssuesPanel(ed) {
  const head = h('h3');
  const list = h('ul', { class: 'ed-issues' });
  const seats = h('ul', { class: 'ed-seats' });
  const el = h('section', { class: 'ed-card', 'aria-label': 'Problems' }, head, list, h('h4', { text: 'Seats' }), seats);

  const jump = (issue) => {
    const room = ed.layout.rooms.find((r) => r.id === issue.id && r.level === 0);
    const furn = ed.layout.furniture.find((f) => f.id === issue.id && f.level === 0);
    ed.setTool('select');
    if (room) ed.select({ kind: 'room', id: room.id });
    else if (furn) ed.select({ kind: 'furn', id: furn.id });
  };

  function refresh() {
    const { errors, warnings, issues } = ed.result;
    head.textContent = issues.length
      ? errors.length + ' problem' + (errors.length === 1 ? '' : 's') + (warnings.length ? ', ' + warnings.length + ' warning' + (warnings.length === 1 ? '' : 's') : '')
      : 'No problems';
    head.className = errors.length ? 'ed-bad' : '';
    list.replaceChildren(...issues.slice(0, 40).map((i) => h('li', { class: i.sev === 'error' ? 'ed-bad' : 'ed-warn' },
      i.id ? h('button', { type: 'button', class: 'ed-link', text: i.msg, onclick: () => jump(i) }) : h('span', { text: i.msg }))));
    const rooms = ed.layout.rooms.filter((r) => r.level === 0);
    seats.replaceChildren(...rooms.map((r) => {
      const n = ed.seats.get(r.id) || 0;
      const short = ed.result.warn.has(r.id);
      const sc = ed.opts.sessionCount;
      const need = r.type === 'cabins' && sc != null ? (typeof sc === 'function' ? sc(r) : sc) : null;
      const tail = need == null ? '' : ' for ' + need + (need === 1 ? ' session' : ' sessions');
      return h('li', { class: short ? 'ed-warn' : '' }, r.label + ': ' + n + (n === 1 ? ' seat' : ' seats') + tail);
    }));
  }
  ed.on('change', refresh);
  ed.on('history', refresh);
  refresh();
  return { el, refresh };
}
