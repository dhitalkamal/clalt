// walls between rooms on the deck. pure, cell coordinates.
// two rooms that meet share a solid wall. when the deck runs on past a room's front edge, that strip is a corridor:
// the room gets a glass front with a door onto it, and a wall shared with another corridor room is closed end to
// end, so nobody crosses one room to reach the next. a room that reaches the deck front instead keeps a doorway at
// the front end of its shared wall. closed lists the walk cells along each wall that agents may not cross.
import { MEZ } from './geom.js';

const DOOR = 2;
// top of a doorway: the lintel above it closes the wall up to the deck's own wall height
const DOOR_TOP = 1.0;

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

// a room whose front edge stops inside the deck opens onto a corridor
const onCorridor = (r, deck) => !!deck && r.z + r.d < deck.z + deck.d && r.x >= deck.x && r.x + r.w <= deck.x + deck.w;

// the door of a room's front: the free gap nearest the middle, else the middle. free(x, z) says whether a cell
// just inside the room is clear of furniture
function frontDoor(r, free) {
  const mid = r.x + (r.w - DOOR) / 2;
  let best = null;
  for (let s = r.x; s + DOOR <= r.x + r.w; s++) {
    let ok = true;
    for (let x = s; x < s + DOOR; x++) if (!free(x, r.z + r.d - 1)) ok = false;
    if (ok && (best === null || Math.abs(s - mid) < Math.abs(best - mid))) best = s;
  }
  const s = best === null ? Math.floor(mid) : best;
  return [s, s + DOOR];
}

const inDoor = (w, c) => w.door && c >= w.door[0] && c < w.door[1];

export function deckWalls(layout, free = () => true) {
  const deck = layout.deck;
  const rooms = (layout.rooms || []).filter((r) => r.level === 1);
  const walls = [];
  const closed = [];
  for (let i = 0; i < rooms.length; i++) {
    for (let j = i + 1; j < rooms.length; j++) {
      const e = sharedEdge(rooms[i], rooms[j]);
      if (!e) continue;
      if (onCorridor(rooms[i], deck) && onCorridor(rooms[j], deck)) {
        walls.push({ ...e, door: null, look: 'solid' });
      } else if (e.to - e.from > DOOR) {
        walls.push({ ...e, door: [e.to - DOOR, e.to], look: 'solid' });
      } else continue;
      const w = walls[walls.length - 1];
      // the cells just inside the first room along the wall
      for (let c = w.from; c < w.to; c++) {
        if (!inDoor(w, c)) closed.push(w.axis === 'x' ? { x: w.at - 1, z: c } : { x: c, z: w.at - 1 });
      }
    }
  }
  for (const r of rooms) {
    if (!onCorridor(r, deck)) continue;
    const w = { axis: 'z', at: r.z + r.d, from: r.x, to: r.x + r.w, door: frontDoor(r, free), look: 'glass' };
    walls.push(w);
    // the corridor row in front of the glass, so the inside of the room keeps all its cells
    for (let x = w.from; x < w.to; x++) if (!inDoor(w, x)) closed.push({ x, z: w.at });
  }
  return { walls, closed };
}

// the pieces of one deck wall along its edge, with world heights above the deck: wall runs as tall as the building
// walls so neither room sees into the other, and a lintel closes the wall over the doorway
export function wallParts(w) {
  const full = (from, to) => ({ from, to, y0: 0, y1: MEZ, kind: 'wall' });
  if (!w.door) return [full(w.from, w.to)];
  const out = [];
  if (w.door[0] > w.from) out.push(full(w.from, w.door[0]));
  if (w.to > w.door[1]) out.push(full(w.door[1], w.to));
  out.push({ from: w.door[0], to: w.door[1], y0: DOOR_TOP, y1: MEZ, kind: 'lintel' });
  return out;
}
