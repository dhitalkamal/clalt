// a coarse spatial hash over line segments, so "how close is this point to any path or river" stays cheap on
// layouts with a hundred paths. pure, no three.js.
import { distSeg } from './geo.js';

const CELL = 2;

export class SegIndex {
  // reach: the largest clearance anyone will ask about, in world units
  constructor(reach = 3) {
    this.reach = reach;
    this.segs = [];
    this.cells = new Map();
  }

  // adds a polyline with a width (the half width counts as inside)
  add(pts, w = 0) {
    for (let i = 0; i + 1 < pts.length; i++) {
      const a = pts[i];
      const b = pts[i + 1];
      const id = this.segs.length;
      this.segs.push({ a, b, h: w / 2 });
      const m = this.reach + w / 2;
      const x0 = Math.floor((Math.min(a[0], b[0]) - m) / CELL);
      const x1 = Math.floor((Math.max(a[0], b[0]) + m) / CELL);
      const z0 = Math.floor((Math.min(a[1], b[1]) - m) / CELL);
      const z1 = Math.floor((Math.max(a[1], b[1]) + m) / CELL);
      for (let x = x0; x <= x1; x++) {
        for (let z = z0; z <= z1; z++) {
          const k = x + ',' + z;
          const list = this.cells.get(k);
          if (list) list.push(id); else this.cells.set(k, [id]);
        }
      }
    }
    return this;
  }

  // distance to the nearest line edge: negative inside a line's width. Infinity beyond reach.
  gap(x, z) {
    const list = this.cells.get(Math.floor(x / CELL) + ',' + Math.floor(z / CELL));
    let best = Infinity;
    if (!list) return best;
    for (const id of list) {
      const s = this.segs[id];
      const d = distSeg(x, z, s.a, s.b) - s.h;
      if (d < best) best = d;
    }
    return best;
  }
}
