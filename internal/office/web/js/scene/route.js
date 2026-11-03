// the walking network of one compiled layout: nodes (a seat, a spot, the door), paths between them over the
// level grids and the stairs, and a flood from the door that says which cells people can reach.
import { isOpen, findPath, nearestOpen } from './nav.js';

let nextId = 1;

export function makeRouter({ grids, f, stairs, entry, door, cellOf, levels }) {
  const cache = new Map();
  const centre = (c) => ({ x: f.X(c.x + 0.5), z: f.Z(c.z + 0.5) });
  const pt = (p, level) => ({ x: p.x, z: p.z, y: levels[level] });

  // a place agents can be: ap is where it meets the open floor, inner leads from there to the exact spot
  function node(level, ap, inner = []) {
    const g = grids[level];
    const raw = cellOf(ap);
    const cell = nearestOpen(g, raw, 3) || raw;
    const last = inner.length ? inner[inner.length - 1] : ap;
    return { id: nextId++, x: last.x, z: last.z, y: levels[level], level, ap, cell, inner };
  }

  const spawn = node(0, centre(entry), [door.th, door.outside]);

  // the stairs: entry cell at the foot, points up the flight and the landing cell
  let st = null;
  if (stairs) {
    const a = stairs.dir;
    const px = -a[1];
    const pz = a[0];
    const foot = [0.25, -0.25].map((k) => cellOf({ x: stairs.B.x - a[0] * 0.25 + px * k, z: stairs.B.z - a[1] * 0.25 + pz * k }));
    const entryCell = foot.find((c) => isOpen(grids[0], c.x, c.z)) || foot[0];
    const land = nearestOpen(grids[1], cellOf(stairs.landingMid), 3) || cellOf(stairs.landingMid);
    st = { entry: entryCell, land, up: [pt(stairs.B, 0), pt(stairs.T, 1), pt(stairs.landingMid, 1)] };
  }

  function walk(level, from, to) {
    const cells = findPath(grids[level], from, to);
    if (!cells) return null;
    return cells.map((c) => ({ ...centre({ x: c.x - 0.5, z: c.z - 0.5 }), y: levels[level] }));
  }

  const sameCell = (n) => { const c = cellOf(n.ap); return c.x === n.cell.x && c.z === n.cell.z; };

  // points between two nodes, both ends included
  function route(a, b) {
    const out = [pt(a, a.level)];
    for (let i = a.inner.length - 2; i >= 0; i--) out.push(pt(a.inner[i], a.level));
    if (a.inner.length) out.push(pt(a.ap, a.level));
    const middle = [];
    // the walk's end cells are dropped when the node's own point stands in them; a point inside furniture was moved
    // to the nearest open cell, and that cell is kept so nobody heads straight for a far waypoint through the furniture
    const from = sameCell(a) ? 1 : 0;
    const cut = (w) => (sameCell(b) ? w.slice(0, -1) : w);
    if (a.level === b.level) {
      const w = walk(a.level, a.cell, b.cell);
      if (w) middle.push(...cut(w.slice(from)));
    } else if (st) {
      const up = a.level === 0;
      const w1 = up ? walk(0, a.cell, st.entry) : walk(1, a.cell, st.land);
      const w2 = up ? walk(1, st.land, b.cell) : walk(0, st.entry, b.cell);
      if (w1) middle.push(...w1.slice(from));
      middle.push(...(up ? st.up : [...st.up].reverse()));
      if (w2) middle.push(...cut(w2));
    }
    out.push(...middle);
    out.push(pt(b.ap, b.level));
    for (const p of b.inner) out.push(pt(p, b.level));
    // drop repeats so an agent never waits on a zero length leg
    return out.filter((p, i) => i === 0 || Math.hypot(p.x - out[i - 1].x, p.z - out[i - 1].z) > 1e-3 || Math.abs(p.y - out[i - 1].y) > 1e-3);
  }

  function path(a, b) {
    const key = a.id + '>' + b.id;
    let r = cache.get(key);
    if (!r) { r = route(a, b); cache.set(key, r); }
    return r;
  }

  // cells reachable from the door on each level
  function flood() {
    const seen = [new Set(), new Set()];
    const run = (level, start) => {
      const g = grids[level];
      const stack = [start];
      seen[level].add(start.x + ',' + start.z);
      while (stack.length) {
        const c = stack.pop();
        for (let dx = -1; dx <= 1; dx++) {
          for (let dz = -1; dz <= 1; dz++) {
            const x = c.x + dx;
            const z = c.z + dz;
            if ((!dx && !dz) || !isOpen(g, x, z) || seen[level].has(x + ',' + z)) continue;
            if (dx && dz && (!isOpen(g, c.x + dx, c.z) || !isOpen(g, c.x, c.z + dz))) continue;
            seen[level].add(x + ',' + z);
            stack.push({ x, z });
          }
        }
      }
    };
    run(0, entry);
    if (st && seen[0].has(st.entry.x + ',' + st.entry.z) && isOpen(grids[1], st.land.x, st.land.z)) run(1, st.land);
    return { has: (level, c) => seen[level].has(c.x + ',' + c.z), seen };
  }

  return { node, spawn, path, flood };
}
