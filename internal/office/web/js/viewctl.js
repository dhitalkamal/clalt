// wires the template panel, camera bar, url hash and key map to the page.
// the layout watcher owns loading; this file owns what the user sees change.
import { app, ui } from './app.js';
import { applyLayout } from './scene/apply.js';
import { startEditor } from './editorhook.js';
import { createChrome } from './chrome.js';
import { createLayoutWatcher } from './layoutwatch.js';
import { actionFor } from './keymap.js';
import { goCam, spinCam, safeCam, navAction, onNavChange, resetNav } from './camseam.js';
import { parseHash, buildHash, remember, orderTemplates, camAfterSwitch } from './viewstate.js';

const JSON_HEADERS = { 'Content-Type': 'application/json', 'X-Clalt-Edit': '1' };
const reduced = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

// the template a layout shows: its own name for a generated one, hall for a v1 file, none for a custom v2
const templateOf = (layout) => (layout.v === 1 ? 'hall' : layout.template || null);

// storage can be missing or blocked, so it is only ever reached through try/catch
function storage() {
  try { return localStorage; } catch (e) { return null; }
}

export function startView(first) {
  const state = { cam: parseHash(location.hash).cam || 'top', spin: false, labels: true, templates: [], edit: false, editor: false, layout: first.layout };
  const saveHash = () => { try { history.replaceState(null, '', buildHash({ template: watcher.template, cam: state.cam }) || location.pathname + location.search); } catch (e) { /* hash stays as it was */ } };
  const notice = (msg) => ui.setNotice(msg);

  const chrome = createChrome({
    template: (id) => pick(id),
    cam: (id) => setCam(id, reduced()),
    spin: toggleSpin,
    labels: toggleLabels,
    makeDefault,
  });
  if (reduced()) chrome.disableSpin('off because the system asks for less motion');

  // a new layout arrived: show it, keep the roster (app.sessions) and the seat plan so agents keep their seats
  async function show(body) {
    state.layout = body.layout;
    state.edit = body.edit;
    if (body.templates.length || !state.templates.length) {
      state.templates = orderTemplates(body.templates);
      chrome.setTemplates(state.templates);
    }
    // the old manager walks the old layout, so it goes before the new one is built
    resetNav();
    state.cam = camAfterSwitch(state.cam);
    await applyLayout(body.layout);
    chrome.showBar();
    chrome.markTemplate(watcher.template || templateOf(body.layout));
    chrome.setEdit(body.edit);
    notice('');
    keepClear();
    chrome.markCam(state.cam);
    goCam(state.cam, true);
    // the editor works on hall layouts only, so it starts the first time a hall shows
    maybeEditor(body);
  }

  // the nav manager reports presets and mode changes: the bar and the url follow
  onNavChange((s) => {
    const id = s.mode === 'ground' ? 'ground' : s.preset || 'sky';
    if (id === state.cam) return;
    state.cam = id;
    chrome.markCam(id);
    saveHash();
  });

  const watcher = createLayoutWatcher({ apply: show });

  // auto-spin is a motion the user must not get under reduced motion
  function toggleSpin() {
    if (reduced()) return;
    state.spin = !state.spin;
    chrome.setSpin(state.spin);
    spinCam(state.spin);
  }

  function toggleLabels() {
    state.labels = !state.labels;
    chrome.setLabels(state.labels);
    ui.layer.hidden = !state.labels;
  }

  function maybeEditor(body) {
    if (!body.edit || body.layout.v !== 1 || state.editor) return;
    state.editor = true;
    startEditor(body.layout).catch((e) => { console.error(e); notice('the editor could not start: ' + e.message); });
  }

  // screen fractions the presets keep free (hall and garden alike): the template panel on the left (wide screens only) and the camera bar
  function keepClear() {
    const wide = innerWidth >= 900;
    safeCam({ left: wide ? 280 / innerWidth : 0, bottom: 70 / Math.max(1, innerHeight), top: 0.02 });
  }

  async function pick(id) {
    if (id === watcher.template) return;
    try {
      await watcher.select(id);
      const s = storage();
      if (s) remember(s, id);
      saveHash();
    } catch (e) { console.error(e); notice('could not load the ' + id + ' template'); }
  }

  function setCam(id, instant) {
    state.cam = id;
    chrome.markCam(id);
    goCam(id, instant);
    saveHash();
  }

  async function makeDefault() {
    const id = watcher.template || templateOf(state.layout);
    if (!id) return;
    chrome.setBusy(true);
    try {
      const res = await fetch('/api/layout/use', { method: 'POST', headers: JSON_HEADERS, credentials: 'same-origin', body: JSON.stringify({ name: id }) });
      if (!res.ok) throw new Error((await res.text()).trim() || 'http ' + res.status);
      notice('');
    } catch (e) { notice('could not make it the default: ' + e.message); } finally { chrome.setBusy(false); }
  }

  watcher.adopt(first.template, first.rev);
  state.templates = orderTemplates(first.templates);
  chrome.setTemplates(state.templates);
  chrome.showBar();
  chrome.markTemplate(first.template || templateOf(first.layout));
  chrome.setEdit(first.edit);
  chrome.markCam(state.cam);
  keepClear();
  goCam(state.cam, true);
  ui.layer.hidden = false;
  maybeEditor(first);

  addEventListener('keydown', (e) => {
    const a = actionFor(e, state.templates, { editing: app.editing });
    if (!a) return;
    if (a.type === 'template') pick(a.id);
    else if (a.type === 'cam') setCam(a.id, reduced());
    else if (a.type === 'spin') toggleSpin();
    else if (['ground', 'view', 'follow', 'escape', 'home'].includes(a.type)) navAction(a.type);
    else if (a.type === 'labels') toggleLabels();
  });
  // the url edited by hand or the back button
  addEventListener('hashchange', () => {
    const h = parseHash(location.hash);
    if (h.template && h.template !== watcher.template) pick(h.template);
    if (h.cam && h.cam !== state.cam) setCam(h.cam, reduced());
  });
  // the panel and the bar sit over the scene: the camera fits the island into what is left
  addEventListener('resize', keepClear);

  return { check: () => watcher.check() };
}
