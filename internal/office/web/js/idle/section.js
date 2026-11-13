// the "When idle" section of the settings dialog: a one-line summary and a button that opens the full list on its
// own notebook page (a dialog over the settings). rows save on change through the shared store.
import { el, clear } from '../dom.js';
import { accountLabel } from '../tool.js';
import { roomsOf } from './resolve.js';
import { summaryText, subtypesOf } from './view.js';
import { choiceSelect } from './select.js';

// h: {store, app}
export function createIdleSection(h) {
  const { store, app } = h;
  const summary = el('p', 'note');
  const open = el('button', '', 'Choose what idle agents do');
  open.type = 'button';
  const root = el('div', 'sec idle-sec', el('h3', '', 'When idle'), summary, open);
  const page = createIdlePage(h);
  open.addEventListener('click', () => page.open(open));

  const paint = () => { summary.textContent = summaryText(store.rules(), roomsOf(app.layoutJson)); };
  store.subscribe(() => { paint(); page.sync(); });
  async function load() {
    await store.load();
    // an office without the route answers 404: the section stays out of the way
    root.hidden = !store.available();
    paint();
  }
  return { el: root, load };
}

// its own page over the settings: Back (or Esc) returns to the settings with focus on the button that opened it
function createIdlePage(h) {
  const { store, app } = h;
  const dlg = el('dialog', 'settings idle-page');
  dlg.setAttribute('aria-labelledby', 'idle-title');
  const title = el('h2', '', 'When idle');
  title.id = 'idle-title';
  const back = el('button', '', 'Back to settings');
  back.type = 'button';
  const err = el('p', 'err');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const body = el('div', 'idle-body');
  dlg.append(el('div', 'idle-head', back, title),
    el('p', 'note', 'choose what each agent does while it has nothing to do. the most specific choice wins: a team member, then the subagent type, then the account, then everyone.'), err, body);
  document.body.append(dlg);
  let opener = null;
  const selects = [];

  const close = () => { dlg.close(); if (opener) opener.focus(); };
  back.addEventListener('click', close);
  dlg.addEventListener('cancel', (e) => { e.preventDefault(); close(); });

  const valueOf = (scope, id) => (id == null ? store.rules()[scope] : store.rules()[scope][id]);
  function sync() { for (const s of selects) s.value = valueOf(s.dataset.scope, s.dataset.id || null) || ''; }

  // a failed save shows one line and the select goes back to what is saved
  async function pick(sel, scope, id) {
    err.hidden = true;
    try { await store.set(scope, id, sel.value); } catch (e) {
      err.textContent = 'could not save: ' + e.message;
      err.hidden = false;
      sync();
    }
  }

  function row(name, sub, scope, id, same) {
    const sel = choiceSelect({ value: valueOf(scope, id), same, rooms: roomsOf(app.layoutJson), label: name + (sub ? ' ' + sub : '') + ' when idle' });
    sel.dataset.scope = scope;
    if (id != null) sel.dataset.id = id;
    sel.addEventListener('change', () => pick(sel, scope, id));
    selects.push(sel);
    const label = el('label', 'idle-name', el('b', '', name), sub ? el('small', '', sub) : null);
    label.htmlFor = sel.id;
    return el('div', 'idle-row', label, sel);
  }

  function fill() {
    clear(body);
    selects.length = 0;
    body.append(row('Everyone', 'sessions and subagents', 'default', null, null), row('All team members', '', 'teamDefault', null, null));
    const group = (head, rows) => {
      if (!rows.length) return;
      body.append(el('h4', '', head), ...rows);
    };
    group('Accounts', [...(app.accounts || new Map())].map(([id, a]) => row(accountLabel(app.accounts, id), 'sessions', 'accounts', id, 'everyone')));
    group('Team members', (app.team || []).map((t) => row(t.name || t.id, '', 'teams', t.id, 'all team members')));
    group('Subagent types', subtypesOf(app.sessions || [], store.rules().subtypes).map((t) => row(t, 'subagents', 'subtypes', t, 'everyone')));
  }

  return {
    open(from) { opener = from; err.hidden = true; fill(); dlg.showModal(); back.focus(); },
    sync,
  };
}
