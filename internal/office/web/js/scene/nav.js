// walkable grids and path finding over them. pure: no three.js. a grid is one level of the layout, cells are
// walkable unless furniture or the stairs block them. paths are lists of world points, smoothed so agents do
// not zigzag between cell centres.
const SQ2 = Math.SQRT2;
const DIRS = [[1, 0, 1], [-1, 0, 1], [0, 1, 1], [0, -1, 1], [1, 1, SQ2], [1, -1, SQ2], [-1, 1, SQ2], [-1, -1, SQ2]];

export function makeGrid(w, d) {
  return { w, d, open: new Uint8Array(w * d) };
}

export const inside = (g, x, z) => x >= 0 && z >= 0 && x < g.w && z < g.d;
export const isOpen = (g, x, z) => inside(g, x, z) && g.open[z * g.w + x] === 1;
export const setOpen = (g, x, z, v) => { if (inside(g, x, z)) g.open[z * g.w + x] = v ? 1 : 0; };

// tiny binary heap keyed by f
class Heap {
  constructor() { this.a = []; }
  push(n) {
    const a = this.a;
    a.push(n);
    let i = a.length - 1;
    while (i > 0) {
      const p = (i - 1) >> 1;
      if (a[p].f <= a[i].f) break;
      [a[p], a[i]] = [a[i], a[p]];
      i = p;
    }
  }
  pop() {
    const a = this.a;
    const top = a[0];
    const last = a.pop();
    if (a.length) {
      a[0] = last;
      let i = 0;
      for (;;) {
        const l = 2 * i + 1;
        const r = l + 1;
        let m = i;
        if (l < a.length && a[l].f < a[m].f) m = l;
        if (r < a.length && a[r].f < a[m].f) m = r;
        if (m === i) break;
        [a[m], a[i]] = [a[i], a[m]];
        i = m;
      }
    }
    return top;
  }
  get size() { return this.a.length; }
}

// a body of radius 0.3 cells passing through (x, z) in cell coordinates must not touch a blocked cell
function clearAt(g, x, z) {
  for (const [dx, dz] of [[0, 0], [0.3, 0], [-0.3, 0], [0, 0.3], [0, -0.3]]) {
    if (!isOpen(g, Math.floor(x + dx), Math.floor(z + dz))) return false;
  }
  return true;
}

// straight walk between two cell-space points is clear of blocked cells
export function lineClear(g, a, b) {
  const n = Math.max(1, Math.ceil(Math.hypot(b.x - a.x, b.z - a.z) / 0.15));
  for (let i = 0; i <= n; i++) {
    const t = i / n;
    const x = a.x + (b.x - a.x) * t;
    const z = a.z + (b.z - a.z) * t;
    const cx = Math.floor(x);
    const cz = Math.floor(z);
    // the start and end cells are allowed to be blocked (a seat inside furniture), everything between must be clear
    if (i === 0 || i === n) continue;
    if (!clearAt(g, x, z) && !(cx === Math.floor(a.x) && cz === Math.floor(a.z)) && !(cx === Math.floor(b.x) && cz === Math.floor(b.z))) return false;
  }
  return true;
}

// A* between two cells. start and goal may be blocked cells (seats), everything else must be open.
// returns cell centres in cell coordinates or null when there is no way.
export function findCells(g, from, to) {
  if (!inside(g, from.x, from.z) || !inside(g, to.x, to.z)) return null;
  const idx = (x, z) => z * g.w + x;
  const best = new Float64Array(g.w * g.d).fill(Infinity);
  const prev = new Int32Array(g.w * g.d).fill(-1);
  const h = (x, z) => Math.hypot(to.x - x, to.z - z);
  const heap = new Heap();
  best[idx(from.x, from.z)] = 0;
  heap.push({ x: from.x, z: from.z, f: h(from.x, from.z) });
  // every cell is expanded at most once per better cost; the cap is a safety net
  let budget = g.w * g.d * 12;
  while (heap.size && budget-- > 0) {
    const n = heap.pop();
    if (n.x === to.x && n.z === to.z) break;
    const gcur = best[idx(n.x, n.z)];
    // a stale entry: a cheaper way to this cell was found after it was queued
    if (n.f - h(n.x, n.z) > gcur + 1e-9) continue;
    for (const [dx, dz, c] of DIRS) {
      const x = n.x + dx;
      const z = n.z + dz;
      const goal = x === to.x && z === to.z;
      if (!inside(g, x, z) || (!goal && !isOpen(g, x, z))) continue;
      // no cutting corners between two blocked cells
      if (dx && dz && (!isOpen(g, n.x + dx, n.z) || !isOpen(g, n.x, n.z + dz))) continue;
      const ng = gcur + c;
      if (ng < best[idx(x, z)]) {
        best[idx(x, z)] = ng;
        prev[idx(x, z)] = idx(n.x, n.z);
        heap.push({ x, z, f: ng + h(x, z) });
      }
    }
  }
  if (!Number.isFinite(best[idx(to.x, to.z)])) return null;
  const out = [];
  for (let i = idx(to.x, to.z); i !== -1; i = prev[i]) out.push({ x: (i % g.w) + 0.5, z: Math.floor(i / g.w) + 0.5 });
  return out.reverse();
}

// string pulling: drop every point the next one can be reached from in a straight line
export function smooth(g, pts) {
  if (pts.length < 3) return pts;
  const out = [pts[0]];
  let at = 0;
  while (at < pts.length - 1) {
    let far = at + 1;
    for (let k = pts.length - 1; k > at + 1; k--) {
      if (lineClear(g, pts[at], pts[k])) { far = k; break; }
    }
    out.push(pts[far]);
    at = far;
  }
  return out;
}

// the closest open cell to a start cell, so a blocked approach still finds the room's floor
export function nearestOpen(g, c, max = 4) {
  if (isOpen(g, c.x, c.z)) return c;
  for (let r = 1; r <= max; r++) {
    for (let dx = -r; dx <= r; dx++) {
      for (let dz = -r; dz <= r; dz++) {
        if (Math.max(Math.abs(dx), Math.abs(dz)) === r && isOpen(g, c.x + dx, c.z + dz)) return { x: c.x + dx, z: c.z + dz };
      }
    }
  }
  return null;
}

export function findPath(g, from, to) {
  const cells = findCells(g, from, to);
  return cells && smooth(g, cells);
}
