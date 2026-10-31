// the settings dialog: pick the default office, open build mode. a modal <dialog>, so the browser
// keeps focus inside, closes on Esc and makes the page behind it inert. the model lives in settings.js.
import { el, clear } from './dom.js';
import { layoutOptions, SAVE_LABEL, confirmText, canSetDefault, AUTOSTART_LABEL, loadAutostart, saveAutostart, templateSketch, canDelete, deletedNote, DELETE_LABEL, CONFIRM_LABEL } from './settings.js';
import { createConfirmPress } from './confirmpress.js';

const SVGNS = 'http://www.w3.org/2000/svg';

const GROUPS = [['template', 'Templates'], ['saved', 'Saved layouts']];
let serial = 0;

// the small line drawing on a layout card. attributes only, the csp allows no inline style
function sketch(id) {
  const svg = document.createElementNS(SVGNS, 'svg');
  svg.setAttribute('viewBox', '0 0 100 58');
  svg.setAttribute('aria-hidden', 'true');
  svg.setAttribute('class', 'sketch');
  const path = document.createElementNS(SVGNS, 'path');
  path.setAttribute('d', templateSketch(id));
  svg.append(path);
  return svg;
}

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

// h: { gear, loadList() -> reply of /api/layouts, save(id) -> promise, remove(id, wasDefault) -> new list, build: the editor's toggle button }
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
  // a round x in the corner, the word stays for screen readers
  const close = el('button', 'x', el('span', 'sr', 'Close'));
  close.type = 'button';
  close.title = 'Close (Esc)';
  const build = h.build;
  // more sections (PR reviews): {el, load()}
  const autostart = createAutostartSection(h.autostart);
  const extra = [...(h.sections || []), autostart];
  // sections for the left page (View): {el, load()}
  const left = h.left || [];
  const buildNote = el('p', 'note');
  // an open notebook: the office on the left page, PR reviews and start at login on the right
  dlg.append(
    close,
    el('div', 'pages',
      el('div', 'page left',
        title,
        el('div', 'sec', el('h3', '', 'Default office'), el('p', 'note', 'what everyone sees when they open the office'), list, el('div', 'row', save), err, ok),
        el('div', 'sec', el('h3', '', 'Build mode'), buildNote, build),
        ...left.map((x) => x.el)),
      el('div', 'page right', ...extra.map((x) => x.el))),
  );
  document.body.append(dlg);

  const setError = (msg) => { err.textContent = msg || ''; err.hidden = !msg; };
  const chosen = () => { const r = list.querySelector('input:checked'); return r ? r.value : null; };
  const setOk = (msg) => { ok.textContent = msg || ''; ok.hidden = !msg; };
  const sync = () => { save.disabled = !canSetDefault(chosen(), current); };
  const busy = (on) => { close.disabled = on; for (const r of list.querySelectorAll('input, button')) r.disabled = on; if (on) save.disabled = true; else sync(); };
  // moves the default tag to the option that now is the default
  const markDefault = (id) => {
    current = id;
    for (const t of list.querySelectorAll('.tag')) t.remove();
    const input = list.querySelector('input[value="' + CSS.escape(id) + '"]');
    if (input) input.parentNode.append(tagFor());
  };
  // the comma is hidden from sight so a screen reader says "River, default"
  const tagFor = () => el('span', 'tag', el('span', 'sr', ', '), 'default');

  // a delete button on a saved card: the first press asks "Delete layout?", the second deletes
  let armed = null;
  const disarm = () => { if (armed) armed.reset(); armed = null; };
  function deleteButton(o) {
    const btn = el('button', 'del', DELETE_LABEL);
    btn.type = 'button';
    const name = 'Delete layout ' + o.title;
    btn.setAttribute('aria-label', name);
    const press = createConfirmPress({
      onChange: (on) => {
        btn.textContent = on ? CONFIRM_LABEL : DELETE_LABEL;
        btn.setAttribute('aria-label', on ? name + '? Press again to confirm' : name);
        btn.classList.toggle('danger', on);
      },
    });
    btn.addEventListener('blur', () => press.reset());
    btn.addEventListener('click', () => {
      // another card's pending delete is dropped first, so it never resets this press
      if (armed !== press) disarm();
      if (press.press() === 'arm') { armed = press; return; }
      armed = null;
      removeLayout(o);
    });
    return btn;
  }

  async function removeLayout(o) {
    setError('');
    setOk('');
    busy(true);
    try {
      fill(await h.remove(o.id, o.id === current));
      setOk(deletedNote(o.title));
    } catch (e) { setError(e.message); } finally { busy(false); }
    // the card is gone: focus goes back to the list
    const first = list.querySelector('input:checked') || list.querySelector('input');
    if (first) first.focus(); else close.focus();
  }

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
        const label = el('label', 'card', input, sketch(o.id), el('span', 'name', o.title));
        titles.set(o.id, o.title);
        if (o.isDefault) label.append(tagFor());
        set.append(canDelete(o) && h.remove ? el('div', 'cardwrap', label, deleteButton(o)) : label);
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
    for (const x of [...left, ...extra]) x.load();
    dlg.showModal();
    try { fill(await h.loadList()); } catch (e) { clear(list); setError('could not load the layouts: ' + e.message); }
    const first = list.querySelector('input:checked') || list.querySelector('input');
    if (first) first.focus(); else close.focus();
  }

  const shut = () => { if (dlg.open) dlg.close(); };
  // Esc and a press elsewhere only disarm a pending delete
  dlg.addEventListener('cancel', (e) => { if (armed) { e.preventDefault(); disarm(); } });
  dlg.addEventListener('pointerdown', (e) => { if (armed && !(e.target.closest && e.target.closest('button.del'))) disarm(); });
  dlg.addEventListener('close', () => { disarm(); h.gear.focus(); });
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
