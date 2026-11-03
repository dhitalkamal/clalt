// the "View" section of the settings dialog: labels, team lines, every tag, auto-spin and the walking camera.
// these were buttons at the bottom of the screen. each switch acts at once, the keys still work. {el, load()}
import { el, clear } from './dom.js';

const ROWS = [
  ['labels', 'Room and account labels', 'L'],
  ['lines', 'Team lines from subagents to their main agent', ''],
  ['tags', 'Show every name tag', ''],
  ['spin', 'Turn the view slowly', 'P'],
  ['ground', 'Walk on the ground', 'G'],
];

// state: {labels, lines, tags, spin, cam, reduced} -> one row per switch
export function viewRows(s) {
  return ROWS.map(([id, label, key]) => {
    const row = { id, label, key, on: false, disabled: false, note: '' };
    if (id === 'ground') row.on = s.cam === 'ground';
    else row.on = !!s[id];
    // auto-spin is a motion the user must not get when the system asks for less of it
    if (id === 'spin' && s.reduced) Object.assign(row, { on: false, disabled: true, note: 'off because the system asks for less motion' });
    return row;
  });
}

let serial = 0;

// h: { get() -> state for viewRows, set(id, on) }
export function createViewSection(h) {
  const list = el('div', 'switches');
  const root = el('div', 'sec view-sec', el('h3', '', 'View'), el('p', 'note', 'what the office shows. the letters are the keys for the same thing'), list);
  const name = 'settings-view-' + ++serial;

  function load() {
    clear(list);
    for (const r of viewRows(h.get())) {
      const box = el('input');
      box.type = 'checkbox';
      box.id = name + '-' + r.id;
      box.checked = r.on;
      box.disabled = r.disabled;
      const text = el('span', '', r.label);
      if (r.key) text.append(' ', el('kbd', '', r.key));
      const label = el('label', '', box, text);
      if (r.note) label.title = r.note;
      box.addEventListener('change', () => { h.set(r.id, box.checked); });
      list.append(label);
    }
  }

  return { el: root, load };
}
