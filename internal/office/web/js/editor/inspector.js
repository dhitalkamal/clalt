// properties of the selection: room, furniture, or the office itself
import { h, field, numberInput, selectInput } from './dom.js';
import { ROOM_TYPES, TYPE_LABELS, updateRoom, setRoomRect, setSize, moveFurniture, rotateFurniture } from './model.js';
import { MIN_ROOM, doorSideLength } from './geometry.js';
import { deleteSelection } from './keys.js';

const SIDES = [['left', 'Left'], ['right', 'Right'], ['back', 'Back'], ['front', 'Front']];

// colour picker: live preview while dragging, one undo step on release
function colorField(ed, text, value, apply) {
  const input = h('input', {
    type: 'color', value,
    oninput: (e) => ed.live((l) => apply(l, e.target.value), 'inspector'),
    onchange: () => ed.commit('inspector'),
  });
  return field(text, input);
}

function textField(ed, text, value, max, apply) {
  return field(text, h('input', {
    type: 'text', value, maxlength: max,
    onchange: (e) => ed.edit((l) => apply(l, e.target.value.trim()), 'inspector'),
  }));
}

function officeForm(ed) {
  const l = ed.layout;
  const door = l.door || { side: 'front', at: 0 };
  const len = doorSideLength(l, door.side);
  return [
    h('h3', { text: 'Office' }),
    textField(ed, 'Title', l.title || '', 60, (lay, v) => (lay.title = v)),
    field('Width (cells)', numberInput(l.size.w, 8, 120, (n) => ed.edit((lay) => setSize(lay, n, lay.size.d), 'inspector'))),
    field('Depth (cells)', numberInput(l.size.d, 8, 120, (n) => ed.edit((lay) => setSize(lay, lay.size.w, n), 'inspector'))),
    colorField(ed, 'Wall colour', l.walls.color, (lay, v) => (lay.walls.color = v)),
    colorField(ed, 'Floor colour', l.floorColor, (lay, v) => (lay.floorColor = v)),
    field('Door side', selectInput(SIDES, door.side, (v) => ed.edit((lay) => (lay.door = { side: v, at: Math.min(lay.door.at, doorSideLength(lay, v) - 1) }), 'inspector'))),
    field('Door position', numberInput(door.at, 0, len - 1, (n) => ed.edit((lay) => (lay.door.at = n), 'inspector'))),
    h('p', { class: 'ed-note', text: 'Tip: pick the Door tool and click a wall edge.' }),
  ];
}

function roomForm(ed, r) {
  const rows = [
    h('h3', { text: 'Room: ' + r.label }),
    field('Type', selectInput(ROOM_TYPES.map((t) => [t, TYPE_LABELS[t]]), r.type, (v) => {
      ed.edit((l) => updateRoom(l, r.id, { type: v }), 'inspector');
      ed.emit('select');
    })),
    textField(ed, 'Label', r.label, 40, (l, v) => updateRoom(l, r.id, { label: v || TYPE_LABELS[r.type] })),
    colorField(ed, 'Floor colour', r.floor || ed.layout.floorColor, (l, v) => updateRoom(l, r.id, { floor: v })),
    h('button', { type: 'button', text: 'Use office floor colour', onclick: () => {
      ed.edit((l) => updateRoom(l, r.id, { floor: '' }));
    } }),
  ];
  if (r.type === 'cabins') {
    const accounts = ed.opts.accounts;
    const control = accounts && accounts.length
      ? selectInput([['', 'Any account'], ...accounts.map((a) => [a.id, a.name || a.id])], r.account || '', (v) => ed.edit((l) => updateRoom(l, r.id, { account: v }), 'inspector'))
      : h('input', { type: 'text', value: r.account || '', maxlength: 40, placeholder: 'any account', onchange: (e) => ed.edit((l) => updateRoom(l, r.id, { account: e.target.value.trim() }), 'inspector') });
    rows.push(field('Account', control));
  }
  const rect = (k, min) => numberInput(r[k], min, 120, (n) => ed.edit((l) => setRoomRect(l, r.id, { ...r, [k]: n }), 'inspector'));
  rows.push(
    field('X', rect('x', 0)), field('Z', rect('z', 0)),
    field('Width', rect('w', MIN_ROOM)), field('Depth', rect('d', MIN_ROOM)),
    h('p', { class: 'ed-note', text: (ed.seats.get(r.id) || 0) + ' seats in this room' }),
    h('button', { type: 'button', class: 'ed-danger', text: 'Delete room', onclick: () => deleteSelection(ed) }));
  return rows;
}

function furnForm(ed, f) {
  const c = ed.catMap.get(f.item);
  const move = (k) => numberInput(f[k], 0, 120, (n) => ed.edit((l) => moveFurniture(l, ed.catMap, f.id, k === 'x' ? n : f.x, k === 'z' ? n : f.z), 'inspector'));
  return [
    h('h3', { text: c ? c.label || c.id : f.item }),
    h('p', { class: 'ed-note', text: 'Facing ' + f.rot + ' degrees' + (f.room ? ', in ' + f.room : ', not in a room') }),
    field('X', move('x')), field('Z', move('z')),
    h('button', { type: 'button', text: 'Rotate (r)', onclick: () => ed.edit((l) => rotateFurniture(l, f.id, ed.catMap)) }),
    h('button', { type: 'button', class: 'ed-danger', text: 'Delete (x)', onclick: () => deleteSelection(ed) }),
  ];
}

export function createInspector(ed) {
  const el = h('section', { class: 'ed-card', 'aria-label': 'Properties' });
  function render() {
    const room = ed.selRoom();
    const furn = ed.selFurn();
    const readOnly = (room && room.level !== 0) || (furn && furn.level !== 0);
    let rows;
    if (readOnly) rows = [h('h3', { text: 'Upper level' }), h('p', { class: 'ed-note', text: 'The upper level is read only in this editor.' })];
    else rows = room ? roomForm(ed, room) : furn ? furnForm(ed, furn) : officeForm(ed);
    el.replaceChildren(...rows);
  }
  ed.on('select', render);
  ed.on('history', render);
  // keep fields current while dragging, but skip rebuilds caused by the form itself
  ed.on('change', (src) => src !== 'inspector' && render());
  render();
  return { el, render };
}
