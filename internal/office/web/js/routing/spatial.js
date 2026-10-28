// grid helpers for routing: footprints, seat points, room lookup, free floor cells.
// all coordinates are cells. furniture x,z is the top-left cell of its footprint.

// catalog may be an array of {id, footprint, tags} or omitted
export function catalogIndex(catalog) {
  const by = new Map();
  for (const c of catalog || []) by.set(c.id, c);
  return by;
}

// rotated footprint in cells: 90 and 270 swap width and depth
export function footprintOf(f, by) {
  const fp = by.get(f.item)?.footprint || f.footprint || { w: 1, d: 1 };
  const swap = f.rot === 90 || f.rot === 270;
  return swap ? { w: fp.d, d: fp.w } : { w: fp.w, d: fp.d };
}

export function tagsOf(f, by) {
  return by.get(f.item)?.tags || f.tags || [];
}

const inRoom = (r, level, px, pz) => r.level === level && px >= r.x && px < r.x + r.w && pz >= r.z && pz < r.z + r.d;

// furniture with a "room" key belongs to that room, otherwise the room holding its center
export function roomOfFurniture(f, fp, rooms, roomById) {
  if (f.room && roomById.has(f.room)) return roomById.get(f.room);
  const cx = f.x + fp.w / 2;
  const cz = f.z + fp.d / 2;
  return rooms.find((r) => inRoom(r, f.level || 0, cx, cz)) || null;
}

// every tagged furniture item as a spot: {id, room, level, x, z, rot}, sorted by z, x, id
export function spotsWithTags(layout, by, wanted) {
  const rooms = layout.rooms || [];
  const roomById = new Map(rooms.map((r) => [r.id, r]));
  const out = [];
  for (const f of layout.furniture || []) {
    if (!tagsOf(f, by).some((t) => wanted.includes(t))) continue;
    const fp = footprintOf(f, by);
    const room = roomOfFurniture(f, fp, rooms, roomById);
    if (!room) continue;
    out.push({ id: f.id, room, level: f.level || 0, x: f.x + fp.w / 2, z: f.z + fp.d / 2, rot: f.rot || 0 });
  }
  out.sort((a, b) => a.z - b.z || a.x - b.x || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
  return out;
}

// cells covered by furniture, as "level,x,z" keys. floor decor (rugs, pergolas) is walked over
export function blockedCells(layout, by) {
  const set = new Set();
  for (const f of layout.furniture || []) {
    if (tagsOf(f, by).includes('floor')) continue;
    const fp = footprintOf(f, by);
    for (let i = 0; i < fp.w; i++) for (let j = 0; j < fp.d; j++) set.add((f.level || 0) + ',' + (f.x + i) + ',' + (f.z + j));
  }
  return set;
}

// free cell centers of a room in row-major order, at most max of them
export function freeCells(room, blocked, max = Infinity) {
  const out = [];
  for (let z = room.z; z < room.z + room.d && out.length < max; z++) {
    for (let x = room.x; x < room.x + room.w && out.length < max; x++) {
      if (blocked.has(room.level + ',' + x + ',' + z)) continue;
      out.push({ level: room.level, x: x + 0.5, z: z + 0.5, rot: 0, room: room.id });
    }
  }
  return out;
}

// where arrivals stand when the layout has no lobby: just inside the door
export function doorCell(layout) {
  const { w, d } = layout.size || { w: 1, d: 1 };
  const door = layout.door || { side: 'left', at: 0 };
  const at = door.at + 0.5;
  const pts = { left: [0.5, at], right: [w - 0.5, at], back: [at, 0.5], front: [at, d - 0.5] };
  const [x, z] = pts[door.side] || pts.left;
  return { level: 0, x, z, rot: 0, room: null };
}
