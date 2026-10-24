// pure cell geometry for the editor. rects are {x, z, w, d} in layout cells.
// a furniture x,z is the top-left cell of its footprint.

export const GRID = 0.5;
export const MIN_ROOM = 2;

export function rotateFootprint(fp, rot) {
  const quarter = rot === 90 || rot === 270;
  return quarter ? { w: fp.d, d: fp.w } : { w: fp.w, d: fp.d };
}

export function nextRot(rot) {
  return (rot + 90) % 360;
}

// footprint rect of a furniture entry, or null when the catalog lacks the item
export function furnitureRect(f, catMap) {
  const c = catMap.get(f.item);
  if (!c) return null;
  const fp = rotateFootprint(c.footprint, f.rot || 0);
  return { x: f.x, z: f.z, w: fp.w, d: fp.d };
}

// touching edges do not count as overlap
export function rectsOverlap(a, b) {
  return a.x < b.x + b.w && b.x < a.x + a.w && a.z < b.z + b.d && b.z < a.z + a.d;
}

export function rectInside(r, outer) {
  return r.w >= 1 && r.d >= 1 && r.x >= outer.x && r.z >= outer.z && r.x + r.w <= outer.x + outer.w && r.z + r.d <= outer.z + outer.d;
}

export function pointInRect(px, pz, r) {
  return px >= r.x && px < r.x + r.w && pz >= r.z && pz < r.z + r.d;
}

export function rectCenterCell(r) {
  return { x: r.x + Math.floor(r.w / 2), z: r.z + Math.floor(r.d / 2) };
}

// two corner cells (inclusive) to a normalized rect
export function rectFromPoints(ax, az, bx, bz) {
  const x = Math.min(ax, bx);
  const z = Math.min(az, bz);
  return { x, z, w: Math.abs(bx - ax) + 1, d: Math.abs(bz - az) + 1 };
}

// move the edges named by the handle (n s e w and corners) by dx, dz cells
export function resizeRect(r, handle, dx, dz, min) {
  let { x, z, w, d } = r;
  if (handle.includes('e')) w = Math.max(min, w + dx);
  if (handle.includes('s')) d = Math.max(min, d + dz);
  if (handle.includes('w')) {
    const m = Math.min(dx, w - min);
    x += m;
    w -= m;
  }
  if (handle.includes('n')) {
    const m = Math.min(dz, d - min);
    z += m;
    d -= m;
  }
  return { x, z, w, d };
}

export function clampRectInto(r, bounds) {
  const x = Math.max(bounds.x, Math.min(r.x, bounds.x + bounds.w - r.w));
  const z = Math.max(bounds.z, Math.min(r.z, bounds.z + bounds.d - r.d));
  return { ...r, x, z };
}

// area of the given level in cells, null when the level does not exist
export function levelBounds(layout, level) {
  if (level === 0) return { x: 0, z: 0, w: layout.size.w, d: layout.size.d };
  return layout.deck ? { ...layout.deck } : null;
}

export function doorSideLength(layout, side) {
  return side === 'left' || side === 'right' ? layout.size.d : layout.size.w;
}

// closest outer wall to a cell, with the offset along it
export function nearestDoor(layout, cx, cz) {
  const { w, d } = layout.size;
  const opts = [
    { side: 'left', dist: cx, at: cz },
    { side: 'right', dist: w - 1 - cx, at: cz },
    { side: 'back', dist: cz, at: cx },
    { side: 'front', dist: d - 1 - cz, at: cx },
  ];
  opts.sort((a, b) => a.dist - b.dist);
  const best = opts[0];
  const max = doorSideLength(layout, best.side) - 1;
  return { side: best.side, at: Math.max(0, Math.min(best.at, max)) };
}

// handle positions of a rect in cell corner coordinates
export const HANDLES = ['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w'];
export function handlePoint(r, h) {
  const cx = r.x + r.w / 2;
  const cz = r.z + r.d / 2;
  const x = h.includes('w') ? r.x : h.includes('e') ? r.x + r.w : cx;
  const z = h.includes('n') ? r.z : h.includes('s') ? r.z + r.d : cz;
  return { x, z };
}

// flight (2 wide, 6 long) and landing (2x2) of stairs, same as the Go
// validator: x,z is the top most cell, dir is the way up (n = smaller z,
// e = larger x), the flight runs the other way and the landing sits past the top
export function stairsRects(s) {
  switch (s.dir) {
    case 'n': return { flight: { x: s.x, z: s.z, w: 2, d: 6 }, landing: { x: s.x, z: s.z - 2, w: 2, d: 2 } };
    case 's': return { flight: { x: s.x, z: s.z - 5, w: 2, d: 6 }, landing: { x: s.x, z: s.z + 1, w: 2, d: 2 } };
    case 'e': return { flight: { x: s.x - 5, z: s.z, w: 6, d: 2 }, landing: { x: s.x + 1, z: s.z, w: 2, d: 2 } };
    default: return { flight: { x: s.x, z: s.z, w: 6, d: 2 }, landing: { x: s.x - 2, z: s.z, w: 2, d: 2 } };
  }
}
