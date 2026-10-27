// the settings dialog: pick the default office, open build mode. a modal <dialog>, so the browser
// keeps focus inside, closes on Esc and makes the page behind it inert. the model lives in settings.js.
import { el, clear } from './dom.js';
import { layoutOptions } from './settings.js';

const GROUPS = [['template', 'Templates'], ['saved', 'Saved layouts']];
let serial = 0;

// h: { gear, loadList() -> reply of /api/layouts, save(id) -> promise, build: the editor's toggle button }
export function createSettingsDialog(h) {
  const dlg = el('dialog', 'settings');
  dlg.setAttribute('aria-labelledby', 'settings-title');
  const title = el('h2', '', 'Settings');
  title.id = 'settings-title';
  const list = el('div', 'choices');
  const err = el('p', 'err');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const save = el('button', 'primary', 'Save');
  save.type = 'button';
  const close = el('button', '', 'Close');
  close.type = 'button';
  const build = h.build;
  const buildNote = el('p', 'note');
  dlg.append(
    title,
    el('div', 'sec', el('h3', '', 'Default office'), el('p', 'note', 'what everyone sees when they open the office'), list, err),
    el('div', 'sec', el('h3', '', 'Build mode'), buildNote, build),
    el('div', 'row', close, save),
  );
  document.body.append(dlg);

  const setError = (msg) => { err.textContent = msg || ''; err.hidden = !msg; };
  const chosen = () => { const r = list.querySelector('input:checked'); return r ? r.value : null; };
  const busy = (on) => { save.disabled = on; close.disabled = on; for (const r of list.querySelectorAll('input')) r.disabled = on; };

  function fill(reply) {
    clear(list);
    const { options } = layoutOptions(reply);
    const name = 'settings-office-' + ++serial;
    for (const [group, title] of GROUPS) {
      const items = options.filter((o) => o.group === group);
      if (!items.length) continue;
      const set = el('fieldset', 'group', el('legend', '', title));
      for (const o of items) {
        const input = el('input');
        input.type = 'radio';
        input.name = name;
        input.value = o.id;
        input.checked = o.checked;
        input.id = name + '-' + o.id;
        const label = el('label', '', input, el('span', '', o.title));
        set.append(label);
      }
      list.append(set);
    }
    if (!options.length) list.append(el('p', 'note', 'no layouts to choose from'));
    save.disabled = !options.length;
  }

  async function open(note) {
    setError('');
    buildNote.textContent = note.text;
    build.disabled = !note.ok;
    clear(list);
    list.append(el('p', 'note', 'loading'));
    save.disabled = true;
    dlg.showModal();
    try { fill(await h.loadList()); } catch (e) { clear(list); setError('could not load the layouts: ' + e.message); }
    const first = list.querySelector('input:checked') || list.querySelector('input');
    if (first) first.focus(); else close.focus();
  }

  const shut = () => { if (dlg.open) dlg.close(); };
  dlg.addEventListener('close', () => h.gear.focus());
  close.addEventListener('click', shut);
  // the editor opens on the same click, so the dialog only has to get out of the way
  build.addEventListener('click', shut);
  save.addEventListener('click', async () => {
    const id = chosen();
    if (!id) return;
    setError('');
    busy(true);
    try { await h.save(id); shut(); } catch (e) { setError(e.message); } finally { busy(false); }
  });

  return { open, isOpen: () => dlg.open };
}
