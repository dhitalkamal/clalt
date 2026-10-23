// the one scene: stage, world group, the app state and the pieces that rebuild when the roster changes
import * as THREE from 'three';
import { CHARACTERS } from './assets.js';
import { createStage, frameRoom } from './stage.js';
import { buildShell } from './room.js';
import { buildGround } from './ground.js';
import { buildZones } from './zones.js';
import { Agent } from './agent.js';
import { computeLayout } from './layout.js';
import { buildLife } from './life.js';
import { UI, renderWing } from './ui.js';
import { disposeTree } from './dispose.js';

// takes a built group out of the world and frees what it held
function drop(group) {
  if (!group) return;
  world.remove(group);
  disposeTree(group);
}

export const canvas = document.getElementById('c');
export const stage = createStage(canvas);
export const ui = new UI(stage.camera, canvas);
export const world = new THREE.Group();
stage.scene.add(world);
const lampLights = [];

export const app = {
  sessions: [], agents: new Map(), layout: null, shell: null, zones: null, hover: null, time: 0, looks: 0,
  lines: new Map(), linesOn: false, tagsAll: false, ticks: [], dyn: { balls: [] }, life: null, hotRef: { until: 0 },
  glance: null, pairCycle: 0, printer: null, selected: null, selectedRegion: null, selectPos: { x: 0, y: 0 },
  dragging: false, accounts: new Map(), hidden: 0,
  occupant(name) { for (const a of this.agents.values()) if (a.mode === 'leisure' && a.curSpot === name) return a; return null; },
};

export function resize() {
  stage.renderer.setSize(innerWidth, innerHeight, false);
  stage.camera.aspect = innerWidth / innerHeight;
  stage.camera.updateProjectionMatrix();
  if (app.layout) frameRoom(stage, app.layout.W, app.layout.D, innerWidth / innerHeight);
}

export const render = () => stage.renderer.render(stage.scene, stage.camera);

// wing and room name tags that float over the scene
export function refreshLabels() {
  const lay = app.layout;
  if (!lay) return;
  const labels = lay.wings.map((w) => {
    const info = app.accounts.get(w.account) || { label: w.account, limits: null };
    return { id: 'w-' + w.account, cls: 'zone', key: JSON.stringify([info.label, info.limits]), x: w.cx, y: 1.25, z: w.z0,
      render: (div) => renderWing(div, info.label, info.limits) };
  });
  const space = (id, text, n, x, y, z) => ({ id, cls: 'space', key: text + n, x, y, z, render: (d) => {
    d.append(text);
    if (n != null) { const s = document.createElement('span'); s.className = 'n'; s.textContent = String(n); d.append(s); }
  } });
  for (const sp of lay.spaces) labels.push(space('sp-' + sp.id, sp.label, sp.n, sp.cx, 0.75, sp.wz0 + 0.05));
  labels.push(space('sp-lobby', 'Lobby', null, (lay.lobby.wx0 + lay.lobby.wx1) / 2, 0.5, lay.lobby.wz0 + 0.2));
  ui.setLabels(labels);
}

// rebuilds the floor plan when its size changes and the cabins and desks every time
export function relayout() {
  const prev = app.layout?.key;
  const lay = computeLayout(app.sessions);
  app.layout = lay;
  if (lay.key !== prev) {
    drop(app.shell);
    for (const l of lampLights) stage.scene.remove(l);
    lampLights.length = 0;
    app.shell = buildShell(lay);
    app.shell.add(buildGround(lay));
    world.add(app.shell);
    drop(app.life?.group);
    app.life = buildLife(lay);
    world.add(app.life.group);
    for (const p of app.shell.userData.lamps) {
      const l = new THREE.PointLight(0xffb36b, 1.4, 3.2, 1.6);
      l.position.copy(p);
      stage.scene.add(l);
      lampLights.push(l);
    }
    frameRoom(stage, lay.W, lay.D, innerWidth / innerHeight);
  }
  drop(app.zones);
  lay.hotRef = app.hotRef;
  const z = buildZones(lay, app.sessions);
  app.printer = z.printer;
  app.zones = z.group;
  app.ticks = z.ticks;
  app.dyn = z.dyn;
  world.add(app.zones);
  refreshLabels();
}

const nextLook = () => CHARACTERS[(app.looks++ * 5) % CHARACTERS.length];

// instant puts the agent in its seat already, for what was running before the page opened
export function makeAgent(s, instant) {
  const slot = app.layout.slots.get(s.id);
  const a = new Agent(s, world, slot, nextLook());
  a.L = app.layout;
  a.env = app;
  app.agents.set(s.id, a);
  if (instant) {
    a.scale = 1;
    a.rig.scale.setScalar(1);
    if (s.state !== 'ended') {
      a.startWalkIn();
      a.sit();
      if (s.state === 'needs-you' && slot.stand) a.instantStand();
    }
  }
  return a;
}

export function lampTick(t) {
  for (let i = 0; i < lampLights.length; i++) lampLights[i].intensity = 1.4 + Math.sin(t * 1.1 + i) * 0.05;
}
