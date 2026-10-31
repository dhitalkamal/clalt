// top bar: tools, room type, undo/redo, menus, save and the status line
import { h, selectInput } from './dom.js';
import { ROOM_TYPES, TYPE_LABELS, cloneLayout } from './model.js';
import { createMenu } from './menus.js';
import { deletedNote } from '../settings.js';

const TOOLS = [
  ['select', 'Select', '1'],
  ['room', 'Room', '2'],
  ['furniture', 'Furniture', '3'],
  ['door', 'Door', '4'],
];

export function createToolbar(ed, actions) {
  const opts = ed.opts;
  const toolBtns = TOOLS.map(([id, text, key]) => h('button', {
    type: 'button', 'data-tool': id, title: text + ' tool (' + key + ')',
    onclick: () => ed.setTool(id), text,
  }));
  const typeSel = selectInput(ROOM_TYPES.map((t) => [t, TYPE_LABELS[t]]), ed.roomType, (v) => (ed.roomType = v));
  typeSel.setAttribute('aria-label', 'Room type for new rooms');
  const typeWrap = h('span', { class: 'ed-inline' }, typeSel);
  const undo = h('button', { type: 'button', text: 'Undo', title: 'Undo (ctrl or cmd + z)', onclick: () => ed.undo() });
  const redo = h('button', { type: 'button', text: 'Redo', title: 'Redo (shift + ctrl or cmd + z)', onclick: () => ed.redo() });
  const save = h('button', { type: 'button', class: 'ed-primary', text: 'Save', onclick: actions.save });
  const status = h('span', { class: 'ed-statusline', role: 'status', 'aria-live': 'polite' });

  const menus = [];
  if (opts.templates) {
    menus.push(createMenu(ed, 'Templates', async () => {
      const list = await opts.templates();
      return list.map((t) => ({ key: t.name, text: t.title || t.name, hint: t.name, layout: t.layout }));
    }, (it) => {
      ed.replaceLayout(cloneLayout(it.layout));
      ed.setStatus('Loaded template ' + it.text + '. Save to keep it, undo to go back.');
    }));
  }
  if (opts.layouts) {
    menus.push(createMenu(ed, 'Layouts', async () => {
      const list = await opts.layouts.list();
      return list.map((t) => ({ key: t.name, text: t.title || t.name, hint: t.active ? 'active' : t.name, removable: !t.builtin, active: t.active }));
    }, async (it) => {
      try {
        const res = await opts.layouts.use(it.key);
        if (res && Array.isArray(res.rooms)) ed.reset(res);
        ed.setStatus('Layout ' + it.text + ' selected.');
      } catch (e) {
        ed.setStatus('Could not switch layout: ' + (e && e.message ? e.message : e), 'bad');
      }
    }, opts.layouts.remove && (async (it) => {
      await opts.layouts.remove(it.key, it.active);
      ed.setStatus(deletedNote(it.text), 'ok');
    })));
  }

  const el = h('div', { class: 'ed-bar', role: 'toolbar', 'aria-label': 'Office editor' },
    h('span', { class: 'ed-group', role: 'group', 'aria-label': 'Tools' }, toolBtns),
    typeWrap,
    h('span', { class: 'ed-group' }, undo, redo),
    menus.map((m) => m.el),
    save,
    status);

  let saving = false;
  const refresh = () => {
    toolBtns.forEach((b) => b.setAttribute('aria-pressed', String(b.dataset.tool === ed.tool)));
    typeWrap.hidden = ed.tool !== 'room';
    undo.disabled = !ed.history.canUndo();
    redo.disabled = !ed.history.canRedo();
    const n = ed.result.errors.length;
    save.disabled = saving;
    save.title = n ? n + ' problem' + (n === 1 ? '' : 's') + ' to fix before saving' : 'Save this layout';
    status.textContent = ed.status.text;
    status.className = 'ed-statusline' + (ed.status.kind ? ' ed-' + ed.status.kind : '');
  };
  ['tool', 'change', 'status', 'history'].forEach((n) => ed.on(n, refresh));
  refresh();
  const setSaving = (v) => {
    saving = v;
    refresh();
  };
  return { el, refresh, setSaving, dispose: () => menus.forEach((m) => m.dispose()) };
}
