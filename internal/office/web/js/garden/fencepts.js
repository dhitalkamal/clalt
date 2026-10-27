// where the picket fence stands: pickets {x, z, ry} and rail runs {x0, z0, x1, z1} along the lawn outlines, with
// gaps where paths, bridges, water or pavilions meet it. pure, shared by the mesh (fence.js) and the walk grid.
import { inPoly, distPoly } from './geo.js';

export const INSET = 0.5;

export function fenceParts(site) {
  const pickets = [];
  const rails = [];
  const skip = (x, z) => site.onRoad(x, z, 0.5)
    || site.bridges.some((b) => Math.hypot(x - (b.a[0] + b.b[0]) / 2, z - (b.a[1] + b.b[1]) / 2) < Math.hypot(b.b[0] - b.a[0], b.b[1] - b.a[1]) / 2 + 1.4)
    || (site.river && site.onWater(x, z))
    || site.pavs.some((it) => distPoly(x, z, it.poly) < 0.45 || inPoly(x, z, it.poly));
  for (const poly of site.land) {
    for (let i = 0; i < poly.length; i++) {
      const p = poly[i];
      const q = poly[(i + 1) % poly.length];
      const len = Math.hypot(q[0] - p[0], q[1] - p[1]);
      if (len < 0.05) continue;
      let nx = -(q[1] - p[1]) / len;
      let nz = (q[0] - p[0]) / len;
      // pick the side that points into the lawn
      if (!inPoly((p[0] + q[0]) / 2 + nx * 0.3, (p[1] + q[1]) / 2 + nz * 0.3, poly)) { nx = -nx; nz = -nz; }
      const ry = Math.atan2(q[0] - p[0], q[1] - p[1]) + Math.PI / 2;
      const n = Math.max(1, Math.floor(len / 0.14));
      let run = null;
      for (let k = 0; k <= n; k++) {
        const t = k / n;
        const x = p[0] + (q[0] - p[0]) * t + nx * INSET;
        const z = p[1] + (q[1] - p[1]) * t + nz * INSET;
        const ok = inPoly(x, z, poly) && !skip(x, z);
        if (ok) pickets.push({ x, z, ry });
        if (ok && !run) run = { x0: x, z0: z };
        if ((!ok || k === n) && run) {
          rails.push({ ...run, x1: x, z1: z });
          run = null;
        }
      }
    }
  }
  return { pickets, rails };
}
