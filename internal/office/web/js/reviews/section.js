// the "PR reviews" section of the settings dialog: on/off, the repos opted in one by one (suggestions come
// from the server), when it last checked. the model is in model.js.
import { el, clear } from '../dom.js';
import { fetchSettings, saveSettings } from './api.js';
import { settingsModel, setEnabled, toggleRepo, addRepo, saveBody } from './model.js';

let serial = 0;

export function createReviewsSection(api = { fetchSettings, saveSettings }) {
  const id = 'pr-reviews-' + ++serial;
  let model = null;
  const on = el('input');
  on.type = 'checkbox';
  on.id = id + '-on';
  const onLabel = el('label', '', on, el('span', '', 'Review pull requests that ask for my review'));
  const note = el('p', 'note', 'off by default. each review runs claude with read-only tools on the account with the most room. nothing is posted to GitHub.');
  const repos = el('fieldset', 'group repos');
  repos.append(el('legend', '', 'Repositories'));
  const list = el('div', 'choices');
  const add = el('input');
  add.type = 'text';
  add.placeholder = 'owner/name';
  add.setAttribute('aria-label', 'add a repository by name');
  add.autocomplete = 'off';
  const addBtn = el('button', '', 'Add');
  addBtn.type = 'button';
  const checked = el('p', 'note');
  const err = el('p', 'err');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const ok = el('p', 'ok');
  ok.setAttribute('role', 'status');
  ok.hidden = true;
  const save = el('button', 'primary', 'Save PR reviews');
  save.type = 'button';
  const root = el('div', 'sec reviews-sec', el('h3', '', 'PR reviews'), onLabel, note, repos, el('div', 'addrow', add, addBtn), checked, err, ok, save);
  repos.append(list);

  const setErr = (m) => { err.textContent = m || ''; err.hidden = !m; };
  const setOk = (m) => { ok.textContent = m || ''; ok.hidden = !m; };

  function draw() {
    on.checked = model.enabled;
    clear(list);
    if (!model.repos.length) list.append(el('p', 'note', 'no repositories found: type one below'));
    model.repos.forEach((r, i) => {
      const box = el('input');
      box.type = 'checkbox';
      box.checked = r.on;
      box.id = id + '-r' + i;
      box.addEventListener('change', () => { model = toggleRepo(model, r.name, box.checked); setOk(''); });
      list.append(el('label', '', box, el('span', '', r.name)));
    });
    checked.textContent = model.lastChecked;
  }

  on.addEventListener('change', () => { model = setEnabled(model, on.checked); setOk(''); });
  addBtn.addEventListener('click', () => {
    const r = addRepo(model, add.value);
    setErr(r.error);
    if (!r.error) { model = r.model; add.value = ''; draw(); }
  });
  add.addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); addBtn.click(); } });
  save.addEventListener('click', async () => {
    setErr('');
    setOk('');
    save.disabled = true;
    try {
      const view = await api.saveSettings(saveBody(model));
      model = settingsModel(view);
      draw();
      setOk(model.enabled ? 'PR reviews are on. The first check runs now.' : 'PR reviews are off.');
    } catch (e) {
      setErr('could not save the PR review settings: ' + e.message);
    } finally {
      save.disabled = false;
    }
  });

  async function load() {
    setErr('');
    setOk('');
    root.hidden = false;
    try {
      model = settingsModel(await api.fetchSettings());
      draw();
      if (model.repos.length === 0) setErr('');
    } catch (e) {
      // an office without the review service answers 404: the section just stays out of the way
      model = null;
      root.hidden = /404|not found/i.test(e.message);
      if (!root.hidden) setErr('could not load the PR review settings: ' + e.message);
    }
    save.disabled = !model;
  }

  return { el: root, load };
}
