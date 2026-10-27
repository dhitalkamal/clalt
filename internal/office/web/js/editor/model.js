// pure edits of a layout object. every function mutates the layout it is
// given (the editor passes its working copy) and returns what it touched.
import { furnitureRect, rectCenterCell, pointInRect, nextRot } from './geometry.js';

export const ROOM_TYPES = ['lobby', 'cabins', 'research', 'workspace', 'review', 'meeting', 'cafe', 'play'];

export const TYPE_LABELS = {
  lobby: 'Lobby', cabins: 'Cabins', research: 'Research', workspace: 'Workspace',
  review: 'Review', meeting: 'Meeting', cafe: 'Cafe', play: 'Play',
};

export const cloneLayout = (l) => JSON.parse(JSON.stringify(l));

export function nextId(list, prefix) {
  const used = new Set(list.map((x) => x.id));
  let n = list.length + 1;
  while (used.has(prefix + n)) n++;
  return prefix + n;
}

export function addRoom(layout, rect, type, level) {
  const room = {
    id: nextId(layout.rooms, 'r'), type, level,
    x: rect.x, z: rect.z, w: rect.w, d: rect.d, label: TYPE_LABELS[type] || type,
  };
  layout.rooms.push(room);
  return room;
}

export const findRoom = (layout, id) => layout.rooms.find((r) => r.id === id);
export const findFurniture = (layout, id) => layout.furniture.find((f) => f.id === id);

// patch fields of a room; empty values drop optional keys, account only on cabins
export function updateRoom(layout, id, patch) {
  const room = findRoom(layout, id);
  if (!room) return null;
  Object.assign(room, patch);
  for (const k of ['floor', 'account']) if (!room[k]) delete room[k];
  if (room.type !== 'cabins') delete room.account;
  return room;
}

export function setRoomRect(layout, id, rect) {
  return updateRoom(layout, id, { x: rect.x, z: rect.z, w: rect.w, d: rect.d });
}

// moves the room and the furniture that belongs to it
export function moveRoom(layout, id, dx, dz) {
  const room = findRoom(layout, id);
  if (!room) return null;
  room.x += dx;
  room.z += dz;
  for (const f of layout.furniture) {
    if (f.room === id) {
      f.x += dx;
      f.z += dz;
    }
  }
  return room;
}

export function removeRoom(layout, id) {
  layout.rooms = layout.rooms.filter((r) => r.id !== id);
  layout.furniture = layout.furniture.filter((f) => f.room !== id);
}

// sets f.room to the room holding the footprint center, or clears it
export function assignRoom(layout, f, catMap) {
  const rect = furnitureRect(f, catMap);
  delete f.room;
  if (!rect) return f;
  const c = rectCenterCell(rect);
  const hit = layout.rooms.find((r) => r.level === f.level && pointInRect(c.x, c.z, r));
  if (hit) f.room = hit.id;
  return f;
}

export function addFurniture(layout, catMap, item, level, x, z, rot) {
  const f = { id: nextId(layout.furniture, 'f'), item, level, x, z, rot };
  assignRoom(layout, f, catMap);
  layout.furniture.push(f);
  return f;
}

export function moveFurniture(layout, catMap, id, x, z) {
  const f = findFurniture(layout, id);
  if (!f) return null;
  f.x = x;
  f.z = z;
  return assignRoom(layout, f, catMap);
}

export function rotateFurniture(layout, id, catMap) {
  const f = findFurniture(layout, id);
  if (!f) return null;
  const before = furnitureRect(f, catMap);
  f.rot = nextRot(f.rot || 0);
  const after = furnitureRect(f, catMap);
  if (before && after) {
    // keep the center put, then re-snap the top-left to the grid
    f.x = Math.round(f.x + before.w / 2 - after.w / 2);
    f.z = Math.round(f.z + before.d / 2 - after.d / 2);
  }
  return assignRoom(layout, f, catMap);
}

export function removeFurniture(layout, id) {
  layout.furniture = layout.furniture.filter((f) => f.id !== id);
}

export function setSize(layout, w, d) {
  const clamp = (n) => Math.max(8, Math.min(120, Math.round(Number(n) || 8)));
  layout.size = { w: clamp(w), d: clamp(d) };
}
