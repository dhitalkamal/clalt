// the quick menu in the activity drawer: one select that edits the rule matching the agent that is open
// (always saved), and a line saying what it saves for. stays in step with the settings page through the store.
import { el } from '../dom.js';
import { roomsOf } from './resolve.js';
import { targetOf, scopeNote } from './view.js';
import { choiceSelect } from './select.js';

// h: {store, app (layout), accountLabel(id) -> string}
export function createIdleQuick(h) {
  const { store } = h;
  const tag = el('label', '', 'When idle');
  const note = el('small');
  const err = el('small', 'idle-err');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const root = el('div', 'idle-quick', tag, note, err);
  root.hidden = true;
  let sel = null;
  let key = '';
  let target = null;
  let savedHere = false;

  const current = () => (target ? store.rules()[target.scope][target.id] || '' : '');
  const sync = () => { if (sel && target) sel.value = current(); };
  store.subscribe(sync);

  // rec is the roster record of the open agent, or null (a run opened from a group, a review)
  function update(rec) {
    target = rec && store.available() ? targetOf(rec, h.accountLabel) : null;
    root.hidden = !target;
    if (!target) { savedHere = false; return; }
    const rooms = roomsOf(h.app.layoutJson);
    const next = [target.scope, target.id, rooms.map((r) => r.id).join(',')].join('|');
    if (next !== key) savedHere = false;
    note.textContent = scopeNote(target, savedHere);
    if (next === key && sel) { sync(); return; }
    key = next;
    err.hidden = true;
    if (sel) sel.remove();
    const t = target;
    sel = choiceSelect({ value: current(), same: t.same, rooms, label: 'When idle' });
    tag.htmlFor = sel.id;
    sel.addEventListener('change', async () => {
      err.hidden = true;
      try {
        await store.set(t.scope, t.id, sel.value);
        savedHere = true;
        note.textContent = scopeNote(t, true);
      } catch (e) {
        err.textContent = 'could not save: ' + e.message;
        err.hidden = false;
        sync();
      }
    });
    tag.after(sel);
  }

  return { el: root, update };
}
