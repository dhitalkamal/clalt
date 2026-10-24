// compiles a layout json (v1) plus the catalog into what the page runs on: items with world positions, the
// walkable grids, seat slots, break spots, regions and the path function. pure, no three.js.
import { MEZ, levels, frame, turned, rotate, yawOf, solidCells } from './geom.js';
import { META, solidRect } from './itemmeta.js';
import { makeGrid, setOpen, isOpen, findCells, nearestOpen } from './nav.js';
import { stairWorld } from './stairs.js';
import { SEAT_OFF } from '../layout.js';
import { makeRouter } from './route.js';

const same = (a, b) => a.x === b.x && a.z === b.z;

// which room an item belongs to: its room key, else the room holding its centre
function roomOf(it, rooms) {
  if (it.rec.room) { const r = rooms.find((x) => x.id === it.rec.room); if (r) return r; }
  const mx = it.rec.x + it.fw / 2;
  const mz = it.rec.z + it.fd / 2;
  return rooms.find((r) => r.level === it.level && mx >= r.x && mx < r.x + r.w && mz >= r.z && mz < r.z + r.d) || null;
}

function doorOf(layout, f) {
  const door = layout.door || { side: 'left', at: 0 };
  const t = Math.floor((door.at || 0) / 2);
  const { size } = layout;
  const side = door.side;
  const along = (side === 'left' || side === 'right' ? f.Z(2 * t + 1) : f.X(2 * t + 1));
  const cells = side === 'left' ? [[0, 2 * t], [0, 2 * t + 1]] : side === 'right' ? [[size.w - 1, 2 * t], [size.w - 1, 2 * t + 1]]
    : side === 'back' ? [[2 * t, 0], [2 * t + 1, 0]] : [[2 * t, size.d - 1], [2 * t + 1, size.d - 1]];
  const out = { left: [-1, 0], right: [1, 0], back: [0, -1], front: [0, 1] }[side];
  const edge = { left: f.xmin, right: f.xmin + f.W, back: f.zmin, front: f.zmin + f.D }[side];
  const horizontal = side === 'left' || side === 'right';
  const th = horizontal ? { x: edge, z: along } : { x: along, z: edge };
  const outside = { x: th.x + out[0] * 0.7, z: th.z + out[1] * 0.7 };
  return { side, tile: t, cells, out, th, outside, along };
}

export function compile(layout, catalog) {
  const by = new Map(catalog.map((c) => [c.id, c]));
  const f = frame(layout.size);
  const rooms = layout.rooms || [];
  const items = [];
  for (const rec of layout.furniture || []) {
    const def = by.get(rec.item);
    if (!def) continue;
    const rot = rec.rot || 0;
    const fp = turned(def.footprint, rot);
    const level = rec.level || 0;
    const it = { rec, def, id: rec.id, level, rot, fw: fp.w, fd: fp.d, y: levels[level], meta: META[def.id] || null,
      cx: f.X(rec.x + fp.w / 2), cz: f.Z(rec.z + fp.d / 2) };
    it.room = roomOf(it, rooms);
    items.push(it);
  }
  const itemById = new Map(items.map((i) => [i.id, i]));
  // world point of a local offset (units) of an item
  const at = (it, lx, lz) => { const r = rotate(lx, lz, it.rot); return { x: it.cx + r.x, z: it.cz + r.z }; };

  // grids
  const grids = [makeGrid(layout.size.w, layout.size.d), makeGrid(layout.size.w, layout.size.d)];
  grids[0].open.fill(1);
  const deck = layout.deck;
  const stairs = layout.stairs && deck ? stairWorld(layout.stairs, f) : null;
  const fillRect = (g, r, v) => { for (let i = 0; i < r.w; i++) for (let j = 0; j < r.d; j++) setOpen(g, r.x + i, r.z + j, v); };
  if (deck) fillRect(grids[1], deck, 1);
  if (stairs) { fillRect(grids[1], stairs.landing, 1); fillRect(grids[0], stairs.flight, 0); }
  for (const it of items) {
    const rect = solidRect(it.def);
    if (!rect) continue;
    for (const c of solidCells(rect, it.def.footprint, it.rot)) setOpen(grids[it.level], it.rec.x + c.i, it.rec.z + c.j, 0);
  }

  const door = doorOf(layout, f);
  const entry = door.cells.map(([x, z]) => ({ x, z })).find((c) => isOpen(grids[0], c.x, c.z)) || nearestOpen(grids[0], { x: door.cells[0][0], z: door.cells[0][1] }, 6)
    || { x: door.cells[0][0], z: door.cells[0][1] };

  const cellOf = (p) => ({ x: Math.floor(f.cx(p.x)), z: Math.floor(f.cz(p.z)) });
  const router = makeRouter({ grids, f, stairs, entry, door, cellOf, levels });
  const node = (level, ap, inner = []) => router.node(level, ap, inner);

  // reachability from the door, so a layout can tell which seats nobody can walk to
  const flood = router.flood();

  // seat slots, one per seat item
  const slotOf = new Map();
  const seatSlot = (it) => {
    if (slotOf.has(it.id)) return slotOf.get(it.id);
    const s = it.meta && it.meta.seat;
    if (!s) return null;
    let rig;
    let chair;
    let rotY = yawOf(it.rot);
    if (s.kind === 'round') {
      // the seat faces the nearest round table, else its own rotation
      chair = { x: it.cx, z: it.cz };
      const tb = items.filter((t) => t.def.id === 'round-table' && t.level === it.level)
        .sort((a, b) => Math.hypot(a.cx - chair.x, a.cz - chair.z) - Math.hypot(b.cx - chair.x, b.cz - chair.z))[0];
      if (tb && Math.hypot(tb.cx - chair.x, tb.cz - chair.z) < 2.5) rotY = Math.atan2(tb.cx - chair.x, tb.cz - chair.z);
      rig = { x: chair.x + Math.sin(rotY) * SEAT_OFF, z: chair.z + Math.cos(rotY) * SEAT_OFF };
    } else {
      rig = at(it, s.rig[0], s.rig[1]);
      chair = at(it, s.rig[0], s.rig[1] - SEAT_OFF);
    }
    const ap = s.kind === 'round' ? chair : at(it, s.approach[0], s.approach[1]);
    const inner = s.inner.map((p) => at(it, p[0], p[1]));
    if (s.kind !== 'round') inner.push({ x: chair.x, z: chair.z });
    const sn = node(it.level, ap, s.kind === 'round' ? [] : inner);
    let stand = s.stand.map((p) => at(it, p[0], p[1]));
    if (s.kind === 'round') {
      const g = grids[it.level];
      const c = cellOf(chair);
      const alt = [[1, 0], [-1, 0], [0, 1], [0, -1]].map(([dx, dz]) => ({ x: c.x + dx, z: c.z + dz })).find((q) => isOpen(g, q.x, q.z));
      stand = alt ? [{ x: f.X(alt.x + 0.5), z: f.Z(alt.z + 0.5) }] : [];
    }
    const slot = { kind: s.kind, side: s.side, y: it.y, x: rig.x, z: rig.z, rotY, chair, node: sn,
      stand: stand.length ? stand : null, corr: node(it.level, ap, []), focus: s.cabin ? { x: it.cx, z: it.cz } : null,
      itemId: it.id, level: it.level, room: it.room ? it.room.id : null, reachable: flood.has(it.level, sn.cell) };
    slotOf.set(it.id, slot);
    return slot;
  };

  // standing slot for an agent without a seat, at a cell centre
  const standSlots = new Map();
  const standSlot = (level, cx, cz) => {
    const key = level + ',' + cx + ',' + cz;
    if (standSlots.has(key)) return standSlots.get(key);
    const p = { x: f.X(cx), z: f.Z(cz) };
    const n = node(level, p, []);
    const slot = { kind: 'stand', side: 1, y: levels[level], x: p.x, z: p.z, rotY: 0, chair: p, node: n, stand: [p], corr: n, focus: null,
      itemId: null, level, room: null, reachable: true };
    standSlots.set(key, slot);
    return slot;
  };

  // break spots from the items that offer them
  const spots = {};
  const groups = {};
  for (const it of items) {
    if (!it.meta || !it.meta.spots) continue;
    for (const sp of it.meta.spots) {
      const p = at(it, sp.at[0], sp.at[1]);
      const name = it.id + ':' + sp.key;
      const face = sp.face ? at(it, sp.face[0], sp.face[1]) : null;
      const yaw = face ? Math.atan2(face.x - p.x, face.z - p.z) : yawOf(it.rot) + (sp.yaw || 0);
      const spot = { name, x: p.x, z: p.z, yaw, act: sp.act, sit: !!sp.sit, level: it.level, item: it,
        partner: sp.pair ? it.id + ':' + sp.pair : undefined, node: node(it.level, p, []) };
      spots[name] = spot;
      (groups[sp.group] = groups[sp.group] || []).push(spot);
    }
  }

  const regions = rooms.map((r) => ({ kind: 'room', id: r.id, type: r.type, label: r.label, x0: f.X(r.x), x1: f.X(r.x + r.w),
    z0: f.Z(r.z), z1: f.Z(r.z + r.d), y: levels[r.level || 0], level: r.level || 0 }));
  const deckW = deck ? { x0: f.X(deck.x), x1: f.X(deck.x + deck.w), z0: f.Z(deck.z), z1: f.Z(deck.z + deck.d), y: MEZ } : null;

  const L = { layout, catalog: by, frame: f, W: f.W, D: f.D, xmin: f.xmin, zmin: f.zmin, X: f.X, Z: f.Z, items, itemById, rooms, grids,
    door, entry, stairs, deck: deckW, regions, spots, groups, seatSlot, standSlot, node, cellOf, spawn: router.spawn, path: router.path, flood,
    key: JSON.stringify([layout.size, layout.door, layout.deck, layout.stairs, layout.walls, layout.floorColor]) };
  L.same = same;
  return L;
}
