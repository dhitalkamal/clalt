// the navigation ctx of a hall (layout v1): a walk with the same surface the ground controller uses, the
// bounds the sky camera clamps to, the box the presets fit, and the stairs as a descriptor. pure, no three.js.
import { isOpen } from '../scene/nav.js';
import { MEZ } from '../scene/geom.js';
import { stairDescOf } from './groundmath.js';

const WALL_TOP = 2.6; // wall height the presets keep in view
const cache = new WeakMap();

// L is the compiled layout from scene/plan.js. the result is cached per compiled layout
export function hallCtx(L) {
  let c = cache.get(L);
  if (c) return c;
  const stairDesc = L.stairs ? stairDescOf({ B: L.stairs.B, T: L.stairs.T, dir: L.stairs.dir, rise: MEZ, width: 1 }) : null;
  // level 0 is the floor, level 1 the deck and its landing. outside the room everything is wall
  const walkable = (x, z, level = 0) => {
    const g = L.grids[level === 1 ? 1 : 0];
    const cell = L.cellOf({ x, z });
    return isOpen(g, cell.x, cell.z);
  };
  const walk = { walkable, spawn: L.spawn, stairDesc, site: null };
  const x1 = L.xmin + L.W;
  const z1 = L.zmin + L.D;
  // seat spots (the chair side of each seat item) for the ground spawn to keep clear of
  const seats = () => L.items.filter((it) => it.def.tags.includes('seat')).map((it) => L.seatSlot(it)).filter(Boolean).map((s) => ({ x: s.x, z: s.z }));
  c = {
    walk, seats,
    stairs: stairDesc,
    bounds: { minX: L.xmin, maxX: x1, minZ: L.zmin, maxZ: z1 },
    bb: { x0: L.xmin, x1, z0: L.zmin, z1, bottom: 0, top: WALL_TOP },
    groundY: (x, z, level = 0) => (level === 1 ? MEZ : 0),
  };
  cache.set(L, c);
  return c;
}
