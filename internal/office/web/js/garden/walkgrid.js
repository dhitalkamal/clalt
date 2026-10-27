// rasterises the walkable ground of a layout: roads (paths, bridges, pavilion floors), the lawn as a fallback and
// the treehouse deck. cells are CELL world units. pure, no three.js.
import { makeGrid, setOpen } from '../scene/nav.js';
import { sizeOf, toWorld } from './footprints.js';
import { rad } from './geo.js';
import { fenceParts } from './fencepts.js';
import { solidsOf, deckLevel, groundLevel, TH_STAIR, TH_X, MEZ } from './local.js';

export const CELL = 0.4;
const FLOOR_MARGIN = 0.3; // floors reach a little past the footprint so a path that stops short still connects
// what props block, as a radius. benches are low and small, agents may brush past them
// a picket closes the cells whose centre is this close, pickets stand 0.14 apart so the line has no holes
const PICKET_R = 0.22;
const PROP_R = { bench: 0.4, lamp: 0.12, fountain: 0.55, 'standing-table': 0.3, sign: 0.15 };

export function makeFrame(bounds) {
  const x0 = bounds.x0 - 2;
  const z0 = bounds.z0 - 2;
  const w = Math.ceil((bounds.x1 - bounds.x0 + 4) / CELL);
  const d = Math.ceil((bounds.z1 - bounds.z0 + 4) / CELL);
  return { x0, z0, w, d, X: (cx) => x0 + cx * CELL, Z: (cz) => z0 + cz * CELL, cx: (x) => (x - x0) / CELL, cz: (z) => (z - z0) / CELL };
}

// a world point into a pavilion's local frame
const toLocal = (p, x, z) => {
  const t = rad(p.rot || 0);
  const c = Math.cos(t);
  const s = Math.sin(t);
  const dx = x - p.x;
  const dz = z - p.z;
  return [dx * c - dz * s, dx * s + dz * c];
};

// visits every cell whose centre may fall in the world box and calls fn(cx, cz, x, z)
function eachCell(f, x0, z0, x1, z1, fn) {
  const c0 = Math.max(0, Math.floor(f.cx(x0)));
  const c1 = Math.min(f.w - 1, Math.ceil(f.cx(x1)));
  const r0 = Math.max(0, Math.floor(f.cz(z0)));
  const r1 = Math.min(f.d - 1, Math.ceil(f.cz(z1)));
  for (let cz = r0; cz <= r1; cz++) for (let cx = c0; cx <= c1; cx++) fn(cx, cz, f.X(cx + 0.5), f.Z(cz + 0.5));
}

// sets every cell inside the local rects and discs of one pavilion to v (1 open, 0 closed). rects flagged wall
// are widened by half a cell so a thin fence still closes the grid
function stamp(g, f, p, shapes, v) {
  const reach = 12;
  eachCell(f, p.x - reach, p.z - reach, p.x + reach, p.z + reach, (cx, cz, x, z) => {
    const [lx, lz] = toLocal(p, x, z);
    for (const r of shapes.rects || []) {
      const e = r.wall ? CELL / 2 : 0;
      if (lx >= r.x0 - e && lx <= r.x1 + e && lz >= r.z0 - e && lz <= r.z1 + e) { setOpen(g, cx, cz, v); break; }
    }
    for (const c of shapes.discs || []) if (Math.hypot(lx - c.x, lz - c.z) <= c.r) { setOpen(g, cx, cz, v); break; }
  });
}

const box = (x0, z0, x1, z1) => ({ x0, z0, x1, z1 });
const TH_PLATFORM = box(TH_X - 4.2, -2.1, TH_X + 4.2, 2.1);
const TH_FLIGHT = box(TH_STAIR.x - 0.5, -2.1, TH_STAIR.x + 0.5, TH_STAIR.foot);

export function buildGrids(layout, site, plan) {
  const f = makeFrame(site.bounds);
  const roads = makeGrid(f.w, f.d);
  const lawn = makeGrid(f.w, f.d);
  const deck = makeGrid(f.w, f.d);
  const b = site.bounds;
  eachCell(f, b.x0, b.z0, b.x1, b.z1, (cx, cz, x, z) => {
    if (site.onRoad(x, z, 0.02)) setOpen(roads, cx, cz, 1);
    if (site.inLand(x, z) || site.onBridge(x, z, 0)) setOpen(lawn, cx, cz, 1);
  });
  // floors
  for (const { p } of site.pavs) {
    if (p.type === 'treehouse') {
      const g = groundLevel();
      for (const grid of [roads, lawn]) stamp(grid, f, p, { rects: g.floors }, 1);
      // the landing is a thin neck into the deck: grow it a cell so a rotated raster cannot pinch it shut
      const [floor, landing] = deckLevel().floors;
      const fat = box(landing.x0 - CELL, landing.z0 - CELL, landing.x1 + CELL, landing.z1 + CELL);
      stamp(deck, f, p, { rects: [floor, fat] }, 1);
      continue;
    }
    const s = sizeOf(p);
    const m = FLOOR_MARGIN;
    stamp(roads, f, p, { rects: [box(-s.w / 2 - m, -s.d / 2 - m, s.w / 2 + m, s.d / 2 + m)] }, 1);
  }
  // blockers last. a bridge deck stays open over the river
  eachCell(f, b.x0, b.z0, b.x1, b.z1, (cx, cz, x, z) => {
    if (site.onWater(x, z)) { setOpen(roads, cx, cz, 0); setOpen(lawn, cx, cz, 0); }
  });
  const disc = (x, z, r) => eachCell(f, x - r, z - r, x + r, z + r, (cx, cz, wx, wz) => {
    if (Math.hypot(wx - x, wz - z) <= r) { setOpen(roads, cx, cz, 0); setOpen(lawn, cx, cz, 0); }
  });
  for (const t of plan.trees) disc(t.x, t.z, Math.max(0.25, Math.min(0.5, 0.3 * t.s)));
  // the fence stops agents everywhere except in its gaps, which fenceParts leaves at paths, bridges and pavilions
  if (layout.site.fence !== false) for (const p of fenceParts(site).pickets) {
    disc(p.x, p.z, PICKET_R);
    // the cell the picket stands in closes too, even when its centre is further off
    const cx = Math.floor(f.cx(p.x));
    const cz = Math.floor(f.cz(p.z));
    for (const g of [roads, lawn]) setOpen(g, cx, cz, 0);
  }
  for (const pr of site.props) {
    if (PROP_R[pr.t]) disc(pr.x, pr.z, PROP_R[pr.t]);
    else if (pr.t === 'bed') disc(pr.x, pr.z, Math.min(0.9, Math.max(pr.w || 1, pr.d || 0.5) / 2));
  }
  for (const { p } of site.pavs) {
    if (p.type === 'treehouse') {
      for (const grid of [roads, lawn]) stamp(grid, f, p, { rects: [TH_PLATFORM, TH_FLIGHT] }, 0);
      // the stair foot sits just past the platform edge and has to stay open
      for (const grid of [roads, lawn]) stamp(grid, f, p, { rects: groundLevel().floors }, 1);
      const dl = deckLevel();
      stamp(deck, f, p, { rects: dl.rects, discs: dl.discs }, 0);
      continue;
    }
    const sol = solidsOf(p);
    stamp(roads, f, p, sol, 0);
    stamp(lawn, f, p, sol, 0);
  }
  return { f, roads, lawn, deck };
}

// world points of the treehouse stairs: foot on the ground, top and landing on the deck. null without a treehouse
export function stairsOf(site) {
  const t = site.pavs.find((x) => x.p.type === 'treehouse');
  if (!t) return null;
  const p = t.p;
  const pt = (lz) => { const [x, z] = toWorld(p, TH_STAIR.x, lz); return { x, z }; };
  const B = pt(TH_STAIR.foot);
  const T = pt(TH_STAIR.top);
  const landing = pt(TH_STAIR.landing + 0.1);
  const len = Math.hypot(T.x - B.x, T.z - B.z) || 1;
  return { B, T, landing, dir: [(T.x - B.x) / len, (T.z - B.z) / len], rise: MEZ };
}
