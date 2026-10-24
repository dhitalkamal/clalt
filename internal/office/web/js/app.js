// the one scene: stage, world group, the app state and the pieces that rebuild when the roster or layout changes.
// the building itself lives in scene/apply.js, which fills app.layout and the groups below.
import * as THREE from 'three';
import { CHARACTERS } from './assets.js';
import { createStage, frameRoom } from './stage.js';
import { Agent } from './agent.js';
import { UI, renderWing } from './ui.js';
import { disposeTree } from './dispose.js';

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
  sessions: [], agents: new Map(), layout: null, layoutJson: null, catalog: null, shell: null, statics: null, dynamic: null, hover: null,
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
    labels.push({ id: 'w-' + (w.room ? w.room.id : '') + '-' + w.account, cls: 'zone', key: JSON.stringify([info.label, info.limits]),
      x: (w.x0 + w.x1) / 2, y: 1.25, z: w.z0, render: (div) => renderWing(div, info.label, info.limits) });
  }
  const used = new Map();
  for (const slot of L.slots ? L.slots.values() : []) if (slot.room) used.set(slot.room, (used.get(slot.room) || 0) + 1);
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

const nextLook = () => CHARACTERS[(app.looks++ * 5) % CHARACTERS.length];

// instant puts the agent in its seat already, for what was running before the page opened
export function makeAgent(s, instant) {
  const slot = app.layout.slots.get(s.id);
  const a = new Agent(s, world, slot, nextLook());
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
