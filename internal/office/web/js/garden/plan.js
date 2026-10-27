// where trees, bushes and lawn tufts go. deterministic for a layout seed, capped for speed, and shared by the
// renderer and the walk grid so agents never walk through a trunk. pure, no three.js.
import { inPoly, distPoly, boundsOf, rng } from './geo.js';
import { buildSite } from './site.js';

export const MAX_TREES = 220;
export const MAX_BUSHES = 90;
export const MAX_TUFTS = 400;

// keeps n items spread over the whole list instead of cutting off the tail
const thin = (list, n) => (list.length <= n ? list : Array.from({ length: n }, (_, i) => list[Math.floor((i * list.length) / n)]));

export function plantPlan(layout, site = buildSite(layout)) {
  const T = { mode: 'edge', band: 3.2, seed: 99, ...(layout.site.trees || {}) };
  const r = rng(T.seed);
  const all = site.land.flat();
  const b = boundsOf(all);
  const step = 1.3;
  const zMid = (b.z0 + b.z1) / 2;
  const edge = (x, z) => {
    let best = null;
    for (const poly of site.land) if (inPoly(x, z, poly)) { const d = distPoly(x, z, poly); if (best === null || d < best) best = d; }
    return best;
  };
  let trees = [];
  const placed = [];
  for (let z = b.z0; z <= b.z1; z += step) {
    for (let x = b.x0; x <= b.x1; x += step * 1.08) {
      const jx = x + (r() - 0.5) * 0.8 + (Math.round((z - b.z0) / step) % 2) * 0.5;
      const jz = z + (r() - 0.5) * 0.7;
      const d = edge(jx, jz);
      if (d === null || d < 0.9) continue;
      // edge mode plants a band along the shore, scatter spreads a sparse wood over the whole lawn
      if (T.mode === 'scatter' ? r() > 0.12 : d > T.band) continue;
      if (!site.free(jx, jz, 0.55)) continue;
      const kind = r() < 0.3 ? 1 : r() < 0.35 ? 2 : 0;
      const s = (jz > zMid + 4 ? 0.8 : 1.0) + r() * 0.35;
      if (placed.some((p) => Math.hypot(p.x - jx, p.z - jz) < 0.62 * (s + p.s) * 0.5 + 0.3)) continue;
      const t = { x: jx, z: jz, kind, s, seed: trees.length };
      trees.push(t);
      placed.push(t);
    }
  }
  trees = thin(trees, MAX_TREES);
  const bushes = [];
  for (let i = 0; i < 70 && bushes.length < MAX_BUSHES; i++) {
    const x = b.x0 + r() * (b.x1 - b.x0);
    const z = b.z0 + r() * (b.z1 - b.z0);
    const d = edge(x, z);
    if (d === null || d < 0.7 || (T.mode !== 'scatter' && d > T.band + 1.5) || !site.free(x, z, 0.3)) continue;
    if (trees.some((p) => Math.hypot(p.x - x, p.z - z) < 0.35)) continue;
    bushes.push({ x, z, s: 0.9 + r() * 0.7, rot: r() * 6, seed: i });
  }
  const tufts = [];
  for (let i = 0; i < 520 && tufts.length < MAX_TUFTS; i++) {
    const x = b.x0 + r() * (b.x1 - b.x0);
    const z = b.z0 + r() * (b.z1 - b.z0);
    const d = edge(x, z);
    if (d === null || d < 0.5 || !site.free(x, z, 0.05)) continue;
    tufts.push({ x, z, s: 0.6 + r() * 0.7, rot: r() * 6 });
  }
  return { trees, bushes, tufts };
}
