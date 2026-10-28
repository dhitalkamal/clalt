// builds the office from a layout json: shell, rooms, furniture, deck, the roster layer and the agents' seats.
// this file is the seam the editor and the page entry call.
import * as THREE from 'three';
import { loadCatalog } from '../catalog.js';
import { app, world, stage, ui, drop, resize, refreshLabels, makeAgent, removeAgent, syncHidden } from '../app.js';
import { frameRoom } from '../stage.js';
import { setBackdrop } from './backdrop.js';
import { createCutaway } from './cutaway.js';
import { buildLife } from '../life.js';
import { SPACE_TINT, flat } from '../zones.js';
import { tintFor } from '../model.js';
import { findRecord, agentRecords } from '../roster.js';
import { compile } from './plan.js';
import { buildShell } from './shell.js';
import { buildDeck } from './deck.js';
import { makeBuilder, placeItem } from './items.js';
import { buildDynamic } from './dynamic.js';
import { levels, cellToWorld as c2w, worldToCell as w2c } from './geom.js';
import { assignSeats } from './seats.js';
import { applyGarden, teardownGarden, isGarden, cellToWorld as gardenCellToWorld, worldToCell as gardenWorldToCell } from '../garden/index.js';

export { levels };
const lights = [];
const DEFAULT_SIZE = { w: 48, d: 20 };

// a garden (layout v2) has no cells: its positions are world units already, so both turn into identity
const garden = () => app.layoutJson && app.layoutJson.v === 2;
export const cellToWorld = (level, x, z) => (garden() ? gardenCellToWorld(level, x, z) : c2w((app.layoutJson || { size: DEFAULT_SIZE }).size, level, x, z));
export const worldToCell = (level, p) => (garden() ? gardenWorldToCell(level, p) : w2c((app.layoutJson || { size: DEFAULT_SIZE }).size, level, p));

const OPACITY = { lobby: 0.22, cafe: 0.22, play: 0.22 };

// rooms get a tint on their floor unless they are cabins (those follow their accounts)
function roomFloors(L, g) {
  for (const r of L.layout.rooms || []) {
    const custom = r.floor ? parseInt(r.floor.slice(1), 16) : null;
    if (r.type === 'cabins' && custom === null) continue;
    const tint = custom !== null ? custom : SPACE_TINT[r.type] || 0xcccccc;
    const level = r.level || 0;
    const op = custom !== null ? 0.3 : level === 1 ? 0.08 : OPACITY[r.type] || 0.26;
    const f = flat(r.w * 0.5, r.d * 0.5, tint, op);
    f.position.set(L.X(r.x + r.w / 2), levels[level] + 0.007, L.Z(r.z + r.d / 2));
    g.add(f);
  }
}

function buildStatic(L) {
  const g = new THREE.Group();
  const b = makeBuilder();
  roomFloors(L, g);
  g.add(buildDeck(L));
  for (const it of L.items) {
    if (it.def.tags.includes('seat')) continue;
    const h = placeItem(it, b, app.hotRef);
    if (h) g.add(h);
  }
  return { group: g, b };
}

// one point light per floor lamp, up to six so the shaders stay cheap
function setLamps(lamps) {
  for (const l of lights) stage.scene.remove(l);
  lights.length = 0;
  for (const obj of lamps.slice(0, 6)) {
    const p = new THREE.Vector3();
    obj.getWorldPosition(p);
    const l = new THREE.PointLight(0xffb36b, 1.4, 3.2, 1.6);
    l.position.set(p.x, p.y + 0.85, p.z);
    stage.scene.add(l);
    lights.push(l);
  }
}

export function lampTick(t) {
  for (let i = 0; i < lights.length; i++) lights[i].intensity = 1.4 + Math.sin(t * 1.1 + i) * 0.05;
}

const recordOf = (key) => findRecord(app.sessions, app.members, key);

// re-plans seats for the current roster, rebuilds the roster layer and re-creates agents whose rig no longer fits
function refreshSeats() {
  const L = app.layout;
  if (!L) return;
  const { slots, plan } = assignSeats(L, app.sessions, app.catalog, app.seatPlan, app.members);
  app.seatPlan = plan;
  L.slots = slots;
  const occ = new Map();
  for (const [key, slot] of slots) {
    const rec = recordOf(key);
    if (!rec) continue;
    if (rec.role === 'sub') rec.space = slot.room ? (L.rooms.find((r) => r.id === slot.room) || {}).type || 'lobby' : 'lobby';
    if (slot.itemId) occ.set(slot.itemId, rec);
  }
  drop(app.dynamic && app.dynamic.group);
  app.dynamic = buildDynamic(L, occ, tintFor);
  world.add(app.dynamic.group);
  const sway = [...(app.statics ? app.statics.b.sway : []), ...app.dynamic.sway];
  app.ticks = [...(app.statics ? app.statics.b.ticks : []), (t) => { for (const s of sway) s.obj.rotation.z = Math.sin(t * 0.8 + s.phase) * 0.03; }];
  syncHidden(slots);
  for (const [id, a] of [...app.agents]) {
    const ns = slots.get(id);
    if (ns && (a.slot.kind !== ns.kind || a.slot.side !== ns.side)) {
      removeAgent(id);
      const rec = recordOf(id);
      if (rec) makeAgent(rec, true);
    }
  }
  refreshLabels();
}

// takes the hall out of the scene, for a v2 layout that draws the garden instead
function clearHall() {
  for (const id of [...app.agents.keys()]) removeAgent(id);
  drop(app.shell);
  drop(app.statics && app.statics.group);
  drop(app.life && app.life.group);
  drop(app.dynamic && app.dynamic.group);
  Object.assign(app, { cutaway: null, shell: null, statics: null, life: null, dynamic: null, layout: null, seatPlan: null, ticks: [], printers: [], pongs: [] });
  setLamps([]);
}

// a new layout: everything is rebuilt and the agents sit down again in their new seats.
// v2 layouts are the garden campus (garden/index.js), v1 layouts are the hall below
export async function applyLayout(layout) {
  if (layout && layout.v === 2) {
    // a garden rebuilding itself keeps its agents and seat plan: only a hall has to be cleared
    if (!isGarden()) clearHall();
    setBackdrop(stage, null);
    return applyGarden(layout);
  }
  teardownGarden();
  const catalog = await loadCatalog();
  const L = compile(layout, catalog);
  const prev = app.layout;
  L.hotRef = app.hotRef;
  app.layout = L;
  app.layoutJson = layout;
  app.catalog = catalog;
  if (!prev || prev.key !== L.key) {
    drop(app.shell);
    app.shell = new THREE.Group();
    const shell = buildShell(L);
    app.shell.add(shell);
    app.cutaway = createCutaway(shell);
    world.add(app.shell);
  }
  setBackdrop(stage, layout);
  drop(app.statics && app.statics.group);
  app.statics = buildStatic(L);
  world.add(app.statics.group);
  app.printers = app.statics.b.printers;
  app.pongs = app.statics.b.pongs;
  setLamps(app.statics.b.lamps);
  drop(app.life && app.life.group);
  app.life = buildLife(L);
  world.add(app.life.group);
  for (const id of [...app.agents.keys()]) removeAgent(id);
  app.dynamic = null;
  app.dimmed = null;
  app.relayoutImpl = refreshSeats;
  refreshSeats();
  for (const rec of agentRecords(app.sessions, app.members)) makeAgent(rec, true);
  if (!prev || prev.W !== L.W || prev.D !== L.D) {
    frameRoom(stage, L.W, L.D, innerWidth / innerHeight, !!stage.nav);
    // a new hall size: the nav camera refits its top view
    if (stage.nav) stage.nav.toPreset('top', { instant: true });
  } else resize();
}
