// the one scene: stage, world group, the app state and the pieces that rebuild when the roster or layout changes.
// the building itself lives in scene/apply.js, which fills app.layout and the groups below.
import * as THREE from 'three';
import { CHARACTERS } from './assets.js';
import { createStage, frameRoom } from './stage.js';
import { Agent } from './agent.js';
import { UI, renderWing, wingKey } from './ui.js';
import { disposeTree } from './dispose.js';
import { pickLook } from './looks.js';
import { agentRecords } from './roster.js';
import { accountLine } from './accountline.js';
import { workingByRoom } from './roomcount.js';

// takes a built group out of the world and frees what it held
export function drop(group) {
  if (!group) return;
  world.remove(group);
  disposeTree(group);
}

export const canvas = document.getElementById('c');
export const stage = createStage(canvas);
export const ui = new UI(stage.camera, canvas);
export const world = new THREE.Group();
stage.scene.add(world);

export const app = {
  sessions: [], members: [], team: [], badges: new Map(), agents: new Map(), layout: null, layoutJson: null, catalog: null, shell: null, statics: null, dynamic: null, hover: null,
  time: 0, looks: 0, lines: new Map(), linesOn: false, tagsAll: false, ticks: [], printers: [], pongs: [], life: null, hotRef: { until: 0 },
  glance: null, pairCycle: 0, selected: null, selectedRegion: null, selectPos: { x: 0, y: 0 }, dragging: false, accounts: new Map(), hidden: 0,
  seatPlan: null, editing: false, relayoutImpl: null, labelsImpl: null, resizeImpl: null,
  occupant(name) { for (const a of this.agents.values()) if (a.mode === 'leisure' && a.curSpot === name) return a; return null; },
};

export function resize() {
  stage.renderer.setSize(innerWidth, innerHeight, false);
  stage.camera.aspect = innerWidth / innerHeight;
  stage.camera.updateProjectionMatrix();
  if (app.layout && app.resizeImpl) app.resizeImpl();
  else if (app.layout) frameRoom(stage, app.layout.W, app.layout.D, innerWidth / innerHeight, !!stage.nav);
}

export const render = () => stage.renderer.render(stage.scene, stage.camera);

// room name pills and account usage cards that float over the scene
export function refreshLabels() {
  const L = app.layout;
  if (!L) return;
  // a garden draws its own labels
  if (app.labelsImpl) { app.labelsImpl(); return; }
  const labels = [];
  for (const w of app.dynamic ? app.dynamic.wings : []) {
    const info = app.accounts.get(w.account) || { label: w.account, limits: null };
    const line = accountLine(app.sessions.filter((s) => !s.leaving && s.account === w.account && (!w.room || L.slots?.get(s.id)?.room === w.room.id))
      .map((s) => ({ state: s.state, away: ['leisure', 'walking'].includes(app.agents.get(s.id)?.mode), rest: !!app.agents.get(s.id)?.plan?.rest })));
    labels.push({ id: 'w-' + (w.room ? w.room.id : '') + '-' + w.account, cls: 'zone', key: JSON.stringify([info.label, info.limits, wingKey(w.account), line]),
      x: (w.x0 + w.x1) / 2, y: 1.25, z: w.z0, render: (div) => renderWing(div, info.label, info.limits, w.account, line) });
  }
  // the number on a room is how many agents work in it
  const states = new Map(agentRecords(app.sessions, app.members).map((r) => [r.id, r.state]));
  const used = workingByRoom(L.slots || new Map(), states);
  for (const r of L.regions) {
    if (r.type === 'cabins') continue;
    const n = r.type === 'lobby' || r.type === 'cafe' || r.type === 'play' ? null : used.get(r.id) || 0;
    const text = r.label || r.type;
    labels.push({ id: 'sp-' + r.id, cls: 'space', key: text + n, x: (r.x0 + r.x1) / 2, y: 0.75 + r.y, z: r.z0 + 0.05, render: (d) => {
      d.append(text);
      if (n != null) { const s = document.createElement('span'); s.className = 'n'; s.textContent = String(n); d.append(s); }
    } });
  }
  ui.setLabels(labels);
}

// roster changed: the scene layer in apply.js re-plans seats and rebuilds the dynamic part
export function relayout() {
  if (app.relayoutImpl) app.relayoutImpl();
}

// a look comes from the record id, so it is the same for the whole session and after a reload
const lookOf = (s) => pickLook(s.id, CHARACTERS);

// instant puts the agent in its seat already, for what was running before the page opened
export function makeAgent(s, instant) {
  const slot = app.layout.slots.get(s.id);
  // an intern past the stools has no slot: no character until a stool frees up
  if (!slot) { s.hid = true; return null; }
  s.hid = false;
  const a = new Agent(s, world, slot, lookOf(s));
  a.L = app.layout;
  a.env = app;
  app.agents.set(s.id, a);
  app.dimmed = null; // the fade of the editor reaches the new agent too
  if (instant) {
    a.scale = 1;
    a.rig.scale.setScalar(1);
    if (s.state !== 'ended') {
      a.startWalkIn();
      a.arrive();
      if (s.state === 'needs-you' && slot.stand && slot.kind !== 'stand') a.instantStand();
    }
  }
  return a;
}

export function removeAgent(id) {
  const a = app.agents.get(id);
  if (!a) return;
  a.dispose();
  app.agents.delete(id);
}

// agents lose their character when the plan hides them (no slot) and get one back when a slot frees up
export function syncHidden(slots) {
  for (const id of [...app.agents.keys()]) {
    if (slots.has(id)) continue;
    const rec = [...app.sessions, ...app.sessions.flatMap((x) => x.subs), ...app.members].find((r) => r.id === id);
    if (rec) rec.hid = true;
    removeAgent(id);
  }
  for (const rec of agentRecords(app.sessions, app.members)) if (rec.hid && slots.has(rec.id) && !app.agents.has(rec.id)) makeAgent(rec, false);
}
