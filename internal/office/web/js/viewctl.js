// wires the settings dialog (with its View switches), url hash and key map to the page.
// the layout watcher owns loading; this file owns what the user sees change.
import { app, ui } from './app.js';
import { applyLayout } from './scene/apply.js';
import { startEditor } from './editorhook.js';
import { createLayoutWatcher } from './layoutwatch.js';
import { actionFor } from './keymap.js';
import { goCam, spinCam, safeCam, navAction, onNavChange, resetNav } from './camseam.js';
import { parseHash, buildHash, camOfNav } from './viewstate.js';
import { runSwitch } from './viewswitch.js';
import { createSettingsDialog } from './settingsdialog.js';
import { saveDefault, deleteLayout } from './settings.js';
import { el } from './dom.js';
import { startReviews } from './reviews/index.js';
import { createViewSection } from './viewsection.js';

const reduced = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

export function startView(first) {
  const state = { cam: parseHash(location.hash).cam || 'sky', spin: false, labels: true, edit: first.edit === true, editor: null, layout: first.layout };
  const saveHash = () => { try { history.replaceState(null, '', buildHash({ cam: state.cam }) || location.pathname + location.search); } catch (e) { /* hash stays as it was */ } };
  const notice = (msg) => ui.setNotice(msg);

  // the switches that used to be buttons at the bottom: they read and flip the same state as the keys
  const view = createViewSection({
    get: () => ({ labels: state.labels, lines: app.linesOn, tags: app.tagsAll, spin: state.spin, cam: state.cam, reduced: reduced() }),
    set: (id, on) => {
      if (id === 'labels' && on !== state.labels) toggleLabels();
      else if (id === 'spin' && on !== state.spin) toggleSpin();
      else if (id === 'lines') app.linesOn = on;
      else if (id === 'tags') app.tagsAll = on;
      else if (id === 'ground') setCam(on ? 'ground' : 'sky', reduced());
    },
  });
  const gear = document.getElementById('b-settings');
  const build = el('button', '', 'Build mode');
  build.type = 'button';
  const reviews = startReviews();
  const dialog = createSettingsDialog({
    gear, build, save: saveAndShow, remove: removeAndShow, sections: [reviews.section], left: [view],
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

  // a new layout arrived: show it, keep the roster (app.sessions) and the seat plan so agents keep their seats
  function show(body) {
    return runSwitch({ resetNav, apply: applyLayout, notice, keepClear, goCam, saveHash, closeEditor, maybeEditor }, state, body);
  }

  function closeEditor() {
    if (state.editor) state.editor.then((api) => { if (api.isOpen()) api.close(); }, () => {});
  }

  // the nav manager reports presets and mode changes: the url follows
  onNavChange((s) => {
    const id = camOfNav(s);
    if (id === state.cam) return;
    state.cam = id;
    saveHash();
  });

  const watcher = createLayoutWatcher({ apply: show });

  // auto-spin is a motion the user must not get under reduced motion
  function toggleSpin() {
    if (reduced()) return;
    state.spin = !state.spin;
    spinCam(state.spin);
  }

  function toggleLabels() {
    state.labels = !state.labels;
    ui.layer.hidden = !state.labels;
  }

  function maybeEditor(body) {
    if (!body.edit || body.layout.v !== 1) return;
    if (state.editor) { state.editor.then((api) => { if (!api.isOpen()) api.setLayout(body.layout); }, () => {}); return; }
    state.editor = startEditor(body.layout, build, afterDelete);
    state.editor.catch((e) => { console.error(e); notice('the editor could not start: ' + e.message); });
  }

  // screen fractions the presets keep free: the panel buttons at the bottom
  function keepClear() {
    safeCam({ left: 0, bottom: 70 / Math.max(1, innerHeight), top: 0.02 });
  }

  // save the default on the server, then show it here without a page load
  async function saveAndShow(id) {
    await saveDefault(id);
    try { await watcher.select(null); } catch (e) { console.error(e); throw new Error('saved, but the page could not load it: ' + e.message); }
  }

  // the active layout was deleted: the server is on the default template now, show it without a reload
  async function afterDelete() {
    try { await watcher.select(null); } catch (e) { console.error(e); throw new Error('deleted, but the page could not load the default: ' + e.message); }
  }

  // delete a saved layout, switch the page when it was the one on show. resolves to the new list
  async function removeAndShow(name, wasActive) {
    const reply = await deleteLayout(name);
    if (wasActive) await afterDelete();
    return reply;
  }

  function setCam(id, instant) {
    state.cam = id;
    goCam(id, instant);
    saveHash();
  }

  watcher.adopt(first.template, first.rev);
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
  // the buttons sit over the scene: the camera fits the island into what is left
  addEventListener('resize', keepClear);

  return { check: () => watcher.check(), reviews: reviews.update };
}
