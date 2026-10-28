// the settings dialog: pick the default office, open build mode. a modal <dialog>, so the browser
// keeps focus inside, closes on Esc and makes the page behind it inert. the model lives in settings.js.
import { el, clear } from './dom.js';
import { layoutOptions, SAVE_LABEL, confirmText, canSetDefault, AUTOSTART_LABEL, loadAutostart, saveAutostart } from './settings.js';

const GROUPS = [['template', 'Templates'], ['saved', 'Saved layouts']];
let serial = 0;

// the "start at login" section: one checkbox that saves on change. {el, load()}
function createAutostartSection(api = { load: loadAutostart, save: saveAutostart }) {
  const box = el('input');
  box.type = 'checkbox';
  box.id = 'settings-autostart-' + ++serial;
  const note = el('p', 'note');
  const err = el('p', 'err');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const root = el('div', 'sec autostart-sec', el('h3', '', 'Start at login'), el('label', '', box, el('span', '', AUTOSTART_LABEL)), note, err);
  const show = (m) => { box.checked = m.enabled; box.disabled = m.disabled; note.textContent = m.note; };
  const fail = (msg) => { err.textContent = msg || ''; err.hidden = !msg; };
  box.addEventListener('change', async () => {
    const want = box.checked;
    fail('');
    box.disabled = true;
    try { show(await api.save(want)); } catch (e) { box.checked = !want; box.disabled = false; fail(e.message); }
  });
  async function load() {
    fail('');
    root.hidden = false;
    show({ enabled: false, disabled: true, note: '' });
    try { show(await api.load()); } catch (e) {
      // an office without the route answers 404: the section stays out of the way
      root.hidden = /404/.test(e.message);
      if (!root.hidden) fail('could not load start at login: ' + e.message);
    }
  }
  return { el: root, load };
}

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
  const ok = el('p', 'ok');
  ok.setAttribute('role', 'status');
  ok.hidden = true;
  let current = '';
  const titles = new Map();
  const save = el('button', 'primary', SAVE_LABEL);
  save.type = 'button';
  const close = el('button', '', 'Close');
  close.type = 'button';
  const build = h.build;
  // more sections (PR reviews): {el, load()}
  const autostart = createAutostartSection(h.autostart);
  const extra = [...(h.sections || []), autostart];
  const buildNote = el('p', 'note');
  dlg.append(
    title,
    el('div', 'sec', el('h3', '', 'Default office'), el('p', 'note', 'what everyone sees when they open the office'), list, err, ok),
    el('div', 'sec', el('h3', '', 'Build mode'), buildNote, build),
    ...extra.map((x) => x.el),
    el('div', 'row', close, save),
  );
  document.body.append(dlg);

  const setError = (msg) => { err.textContent = msg || ''; err.hidden = !msg; };
  const chosen = () => { const r = list.querySelector('input:checked'); return r ? r.value : null; };
  const setOk = (msg) => { ok.textContent = msg || ''; ok.hidden = !msg; };
  const sync = () => { save.disabled = !canSetDefault(chosen(), current); };
  const busy = (on) => { close.disabled = on; for (const r of list.querySelectorAll('input')) r.disabled = on; if (on) save.disabled = true; else sync(); };
  // moves the default tag to the option that now is the default
  const markDefault = (id) => {
    current = id;
    for (const t of list.querySelectorAll('.tag')) t.remove();
    const input = list.querySelector('input[value="' + CSS.escape(id) + '"]');
    if (input) input.parentNode.append(tagFor());
  };
  // the comma is hidden from sight so a screen reader says "River, default"
  const tagFor = () => el('span', 'tag', el('span', 'sr', ', '), 'default');

  function fill(reply) {
    clear(list);
    titles.clear();
    const { options, active } = layoutOptions(reply);
    current = active;
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
        titles.set(o.id, o.title);
        if (o.isDefault) label.append(tagFor());
        set.append(label);
      }
      list.append(set);
    }
    if (!options.length) list.append(el('p', 'note', 'no layouts to choose from'));
    sync();
    for (const r of list.querySelectorAll('input')) r.addEventListener('change', () => { setOk(''); sync(); });
  }

  async function open(note) {
    setError('');
    setOk('');
    buildNote.textContent = note.text;
    build.disabled = !note.ok;
    clear(list);
    list.append(el('p', 'note', 'loading'));
    save.disabled = true;
    for (const x of extra) x.load();
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
    setOk('');
    busy(true);
    try { await h.save(id); markDefault(id); setOk(confirmText(titles.get(id) || id)); } catch (e) { setError(e.message); } finally { busy(false); }
  });

  return { open, isOpen: () => dlg.open };
}
