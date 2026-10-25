// nameplates of the team pavilions: pure text mapping plus one shared canvas atlas (one texture for every plate).
// the atlas is drawn through res.js so a rebuild frees it. setTeamList redraws names without a rebuild.
import * as THREE from 'three';
import { cached, geo } from './res.js';
import { canvasTex } from './tex.js';
import { COLS, ROWS, PLATE_CELLS, plateTexts, plateCell } from './teamtext.js';

export { plateTexts, plateCell, PLATE_CELLS };

const CW = 256;
const CH = 64;
let list = [];

export function setTeamList(team) {
  list = Array.isArray(team) ? team : [];
}

export const teamList = () => list;

// which atlas cell shows which member, filled while the pavilions are built
let slots = new Map();
export const resetPlates = () => { slots = new Map(); };

export function plateSlot(id) {
  if (!slots.has(id)) slots.set(id, slots.size);
  return slots.get(id);
}

function drawCell(c, i, t) {
  const k = i % PLATE_CELLS;
  const x = (k % COLS) * CW;
  const y = Math.floor(k / COLS) * CH;
  c.fillStyle = '#f4efe3';
  c.fillRect(x, y, CW, CH);
  c.fillStyle = t.color;
  c.fillRect(x, y, 18, CH);
  c.fillStyle = '#2a2430';
  c.font = '600 30px system-ui, sans-serif';
  c.textAlign = 'left';
  c.textBaseline = 'middle';
  c.fillText(t.name, x + 30, y + CH / 2 + 1, CW - 40);
}

export function atlas() {
  return cached('t:plates', () => {
    const t = canvasTex(CW * COLS, CH * ROWS, (c, w, h) => { c.fillStyle = '#f4efe3'; c.fillRect(0, 0, w, h); });
    return t;
  });
}

// redraws every member that has a cell. called after the list changes and after the pavilions are built
export function paintPlates(team) {
  if (!slots.size) return;
  const tex = atlas();
  const c = tex.image.getContext('2d');
  for (const [id, i] of slots) drawCell(c, i, plateTexts([id], team)[0]);
  tex.needsUpdate = true;
}

// accent strip material of a member desk, shared so a late team list can recolour it
export const stripMat = (id, color) => cached('m:strip:' + id, () => new THREE.MeshStandardMaterial({ color, roughness: 0.6 }));

// a new team list arrived: redraw the names and recolour the strips, no rebuild. a no-op while no plate exists
let lastKey = '';
export function updateTeam(team) {
  const next = Array.isArray(team) ? team : [];
  const key = JSON.stringify(next.map((t) => [t.id, t.name, t.color]));
  if (key === lastKey) return;
  lastKey = key;
  setTeamList(next);
  if (!slots.size) return;
  paintPlates(next);
  for (const [id] of slots) {
    const t = plateTexts([id], next)[0];
    stripMat(id, t.color).color.set(t.color);
  }
}

export function plateMesh(id) {
  const i = plateSlot(id);
  const g = geo(`plate:${i}`, () => {
    const pg = new THREE.PlaneGeometry(0.34, 0.085);
    const { u0, u1, v0, v1 } = plateCell(i);
    pg.setAttribute('uv', new THREE.Float32BufferAttribute([u0, v1, u1, v1, u0, v0, u1, v0], 2));
    return pg;
  });
  const m = cached('m:plates', () => new THREE.MeshBasicMaterial({ map: atlas(), toneMapped: false }));
  return new THREE.Mesh(g, m);
}
