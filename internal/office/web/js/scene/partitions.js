// glass walls between rooms that meet on the deck. pure, cell coordinates.
// a wall runs along the shared edge and stops short of its far end (the deck front or the right side),
// leaving a doorway of DOOR cells. closed lists the walk cells just before the wall that agents may not cross.
const DOOR = 2;

// the shared edge of two rooms as {axis, at, from, to}, or null. axis 'x' is a wall at x = at running along z
function sharedEdge(a, b) {
  if (a.x + a.w === b.x || b.x + b.w === a.x) {
    const from = Math.max(a.z, b.z);
    const to = Math.min(a.z + a.d, b.z + b.d);
    return to > from ? { axis: 'x', at: Math.max(a.x, b.x), from, to } : null;
  }
  if (a.z + a.d === b.z || b.z + b.d === a.z) {
    const from = Math.max(a.x, b.x);
    const to = Math.min(a.x + a.w, b.x + b.w);
    return to > from ? { axis: 'z', at: Math.max(a.z, b.z), from, to } : null;
  }
  return null;
}

export function deckWalls(layout) {
  const rooms = (layout.rooms || []).filter((r) => r.level === 1);
  const walls = [];
  const closed = [];
  for (let i = 0; i < rooms.length; i++) {
    for (let j = i + 1; j < rooms.length; j++) {
      const e = sharedEdge(rooms[i], rooms[j]);
      if (!e || e.to - e.from <= DOOR) continue;
      const w = { ...e, to: e.to - DOOR };
      walls.push(w);
      for (let c = w.from; c < w.to; c++) closed.push(w.axis === 'x' ? { x: w.at - 1, z: c } : { x: c, z: w.at - 1 });
    }
  }
  return { walls, closed };
}
