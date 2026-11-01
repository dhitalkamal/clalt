// a plan to rest in one room of a hall: one step that never ends by itself. the agent walks to a seat-tagged
// spot of the room (bean bags), else a free floor cell, and stays until the choice or its state changes.
import { isOpen } from '../scene/nav.js';

const FOREVER = 1e9;
const CELLS = 6;

// open, reachable cells of a room, nearest to its middle first
function floorCells(L, room) {
  const g = L.grids && L.grids[room.level];
  if (!g) return [];
  const mx = room.x + room.w / 2;
  const mz = room.z + room.d / 2;
  const out = [];
  for (let z = room.z; z < room.z + room.d; z++) {
    for (let x = room.x; x < room.x + room.w; x++) {
      if (!isOpen(g, x, z) || (L.flood && !L.flood.has(room.level, { x, z }))) continue;
      out.push({ x, z, d: Math.hypot(x + 0.5 - mx, z + 0.5 - mz) });
    }
  }
  return out.sort((a, b) => a.d - b.d);
}

// the floor spot of a cell, made once and kept in L.spots so the break code finds it by name
function floorSpot(L, room, c) {
  const name = 'rest:' + room.id + ':' + c.x + ',' + c.z;
  if (!L.spots[name]) {
    const p = { x: L.X(c.x + 0.5), z: L.Z(c.z + 0.5) };
    L.spots[name] = { name, x: p.x, z: p.z, yaw: 0, act: 'rest', sit: false, level: room.level, item: null, node: L.node(room.level, p, []) };
  }
  return name;
}

// -> {steps: [{spot, act, dur}], carry: false} or null when the room has no free place.
// taken is the set of spot names other agents hold, seed spreads agents over the cells
export function restPlan(L, roomId, taken, seed) {
  const room = (L.rooms || []).find((r) => r.id === roomId);
  if (!room || !L.spots) return null;
  const step = (spot) => ({ steps: [{ spot, act: 'rest', dur: FOREVER }], carry: false, rest: roomId });
  const seat = Object.values(L.spots).find((s) => s.sit && !s.partner && s.item && s.item.room && s.item.room.id === roomId && !taken.has(s.name));
  if (seat) return step(seat.name);
  const cells = floorCells(L, room).slice(0, 64);
  const near = Math.min(cells.length, CELLS);
  for (let i = 0; i < cells.length; i++) {
    const c = cells[(i < near ? (seed + i) % near : i)];
    const name = 'rest:' + room.id + ':' + c.x + ',' + c.z;
    if (!taken.has(name)) return step(floorSpot(L, room, c));
  }
  return null;
}
