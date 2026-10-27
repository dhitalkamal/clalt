// wires the camera bar, settings dialog, url hash and key map to the page.
// the layout watcher owns loading; this file owns what the user sees change.
import { app, ui } from './app.js';
import { applyLayout } from './scene/apply.js';
import { startEditor } from './editorhook.js';
import { createChrome } from './chrome.js';
import { createLayoutWatcher } from './layoutwatch.js';
import { actionFor } from './keymap.js';
import { goCam, spinCam, safeCam, navAction, onNavChange, resetNav } from './camseam.js';
import { parseHash, buildHash, camAfterSwitch } from './viewstate.js';
import { createSettingsDialog } from './settingsdialog.js';
import { saveDefault } from './settings.js';
import { el } from './dom.js';
import { startReviews } from './reviews/index.js';

const reduced = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

export function startView(first) {
  const state = { cam: parseHash(location.hash).cam || 'top', spin: false, labels: true, edit: first.edit === true, editor: null, layout: first.layout };
  const saveHash = () => { try { history.replaceState(null, '', buildHash({ cam: state.cam }) || location.pathname + location.search); } catch (e) { /* hash stays as it was */ } };
  const notice = (msg) => ui.setNotice(msg);

  const chrome = createChrome({
    cam: (id) => setCam(id, reduced()),
    spin: toggleSpin,
    labels: toggleLabels,
  });
  const gear = document.getElementById('b-settings');
  const build = el('button', '', 'Build mode');
  build.type = 'button';
  const reviews = startReviews();
  const dialog = createSettingsDialog({
    gear, build, save: saveAndShow, sections: [reviews.section],
    loadList: async () => {
      const res = await fetch('/api/layouts', { cache: 'no-store', credentials: 'same-origin' });
      if (!res.ok) throw new Error('http ' + res.status);
      return res.json();
    },
  });
  gear.addEventListener('click', () => dialog.open(buildNote()));
  // why build mode is or is not available for the layout on screen
  function buildNote() {
    if (!state.edit) return { ok: false, text: 'the server does not allow editing' };
    if (state.layout.v !== 1) return { ok: false, text: 'build mode edits the hall only: pick Hall as the default office first' };
    return { ok: true, text: 'edit the floor plan of the hall' };
  }
  if (reduced()) chrome.disableSpin('off because the system asks for less motion');

  // a new layout arrived: show it, keep the roster (app.sessions) and the seat plan so agents keep their seats
  async function show(body) {
    state.layout = body.layout;
    state.edit = body.edit;
    // the old manager walks the old layout, so it goes before the new one is built
    resetNav();
    state.cam = camAfterSwitch(state.cam);
    await applyLayout(body.layout);
    chrome.showBar();
    notice('');
    keepClear();
    chrome.markCam(state.cam);
    goCam(state.cam, true);
    // the editor works on hall layouts only: it starts the first time a hall shows and follows later halls
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
    if (!body.edit || body.layout.v !== 1) return;
    if (state.editor) { state.editor.then((api) => { if (!api.isOpen()) api.setLayout(body.layout); }, () => {}); return; }
    state.editor = startEditor(body.layout, build);
    state.editor.catch((e) => { console.error(e); notice('the editor could not start: ' + e.message); });
  }

  // screen fractions the presets keep free: the camera bar at the bottom
  function keepClear() {
    safeCam({ left: 0, bottom: 70 / Math.max(1, innerHeight), top: 0.02 });
  }

  // save the default on the server, then show it here without a page load
  async function saveAndShow(id) {
    await saveDefault(id);
    try { await watcher.select(null); } catch (e) { console.error(e); throw new Error('saved, but the page could not load it: ' + e.message); }
  }

  function setCam(id, instant) {
    state.cam = id;
    chrome.markCam(id);
    goCam(id, instant);
    saveHash();
  }

  watcher.adopt(first.template, first.rev);
  chrome.showBar();
  chrome.markCam(state.cam);
  keepClear();
  goCam(state.cam, true);
  ui.layer.hidden = false;
  maybeEditor(first);

  addEventListener('keydown', (e) => {
    const a = actionFor(e, { editing: app.editing || dialog.isOpen() });
    if (!a) return;
    if (a.type === 'cam') setCam(a.id, reduced());
    else if (a.type === 'spin') toggleSpin();
    else if (['ground', 'view', 'follow', 'escape', 'home'].includes(a.type)) navAction(a.type);
    else if (a.type === 'labels') toggleLabels();
  });
  // the url edited by hand or the back button
  addEventListener('hashchange', () => {
    const h = parseHash(location.hash);
    if (h.cam && h.cam !== state.cam) setCam(h.cam, reduced());
  });
  // the bar sits over the scene: the camera fits the island into what is left
  addEventListener('resize', keepClear);

  return { check: () => watcher.check(), reviews: reviews.update };
}
