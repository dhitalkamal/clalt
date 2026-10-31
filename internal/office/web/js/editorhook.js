// wires the editor into the page: it talks to clalt's layout routes and hands the editor the scene seams
import { mountEditor } from './editor/index.js';
import { app, stage } from './app.js';
import { setPreset } from './stage.js';
import { goCam } from './camseam.js';
import { loadCatalog } from './catalog.js';
import { applyLayout, cellToWorld, worldToCell } from './scene/apply.js';
import { roomTypeFor } from './routing.js';
import { deleteLayout } from './settings.js';

const JSON_HEADERS = { 'Content-Type': 'application/json', 'X-Clalt-Edit': '1' };

// failures come back as a one line text body
async function check(res) {
  if (res.ok) return res;
  const text = (await res.text()).trim();
  throw new Error(text || 'http ' + res.status);
}

const getJSON = async (url) => (await check(await fetch(url, { cache: 'no-store', credentials: 'same-origin' }))).json();

async function save(layout) {
  const res = await fetch('/api/layout', { method: 'PUT', headers: JSON_HEADERS, credentials: 'same-origin', body: JSON.stringify(layout) });
  await check(res);
}

// built-in templates: the layouts the server lists as built in, as the page ships them
async function templates() {
  const { layouts } = await getJSON('/api/layouts');
  const out = [];
  for (const info of layouts.filter((l) => l.builtin)) {
    try {
      out.push({ name: info.name, title: info.title, layout: await getJSON('./layout/' + info.name + '.json') });
    } catch (e) { /* a template the page has no copy of is not offered */ }
  }
  return out;
}

// onSwitched runs after the active layout was deleted and the server fell back to the default
const layouts = (onSwitched) => ({
  async remove(name, wasActive) {
    await deleteLayout(name);
    if (wasActive && onSwitched) await onSwitched();
  },
  async list() {
    const { active, layouts: list } = await getJSON('/api/layouts');
    return list.map((l) => ({ name: l.name, title: l.title || l.name, active: l.name === active, builtin: !!l.builtin }));
  },
  async use(name) {
    const res = await check(await fetch('/api/layout/use', { method: 'POST', headers: JSON_HEADERS, credentials: 'same-origin', body: JSON.stringify({ name }) }));
    const body = await res.json();
    await applyLayout(body.layout);
    return body.layout;
  },
});

// how many live agents want a seat in a room of this type, for the seat count check
function demand(room) {
  const live = app.sessions.filter((s) => !s.leaving);
  if (room.type === 'cabins') return live.filter((s) => !room.account || s.account === room.account).length;
  return live.flatMap((s) => s.subs).filter((x) => !x.leaving && roomTypeFor(x.type) === room.type).length;
}

// button is the settings dialog's Build mode button, it toggles the editor
export async function startEditor(layout, button, onSwitched) {
  const accounts = () => [...app.accounts].map(([id, a]) => ({ id, name: a.label }));
  return mountEditor({
    layout, catalog: await loadCatalog(), applyLayout, cellToWorld, worldToCell, save, button,
    camera: stage.camera, renderer: stage.renderer, templates, layouts: layouts(onSwitched), sessionCount: demand,
    get accounts() { return accounts(); },
    onOpen: () => { app.editing = true; setPreset(stage, 'top'); goCam('top', true); },
    onClose: () => { app.editing = false; },
  });
}
