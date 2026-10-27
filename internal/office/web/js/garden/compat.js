// makes a v2 layout look like the compiled hall layout the agent code expects: seat slots, break spots and
// groups, the walk function, the spawn point, regions for the hover card. names follow scene/plan.js:
// L.seatSlot, L.standSlot, L.spots, L.groups, L.path, L.spawn, L.regions, L.W and L.D.
// cellToWorld and worldToCell are identity for a garden: positions are already world units.
import { gardenWalk } from './walk.js';
import { gardenSeats, gardenOverflow } from './seats.js';
import { rad, boundsOf } from './geo.js';
import { toWorld, polyOf } from './footprints.js';
import { MEZ, SEAT_OFF, TH_X } from './local.js';

const GROUP_OF = { order: 'counter', sip: 'table', pong: 'pong', read: 'bag' };

// rig position of a seat: desks are the desk centre, round seats put the rig one chair offset toward the table
function rigOf(seat) {
  if (seat.kind !== 'round' && seat.kind !== 'review' && seat.kind !== 'meeting' && seat.kind !== 'stool') return { x: seat.x, z: seat.z };
  const t = rad(seat.rot);
  return { x: seat.x + Math.sin(t) * SEAT_OFF, z: seat.z + Math.cos(t) * SEAT_OFF };
}

export function compileGarden(layout) {
  const walk = gardenWalk(layout);
  const seats = gardenSeats(layout);
  const overflow = gardenOverflow(layout);
  const seatById = new Map(seats.map((s) => [s.id, s]));
  const pavById = new Map((layout.pavilions || []).map((p) => [p.id, p]));
  const b = walk.site.bounds;

  const slotOf = new Map();
  function seatSlot(seat) {
    if (slotOf.has(seat.id)) return slotOf.get(seat.id);
    const level = seat.y > 0.5 ? 1 : 0;
    const t = rad(seat.rot);
    const at = (lx, lz) => { const [x, z] = toWorld({ x: seat.x, z: seat.z, rot: seat.rot }, lx, lz); return { x, z }; };
    let slot;
    if (seat.kind === 'desk') {
      const laneX = seat.lane * (seat.cols > 1 ? 1.05 : 0.85);
      const node = walk.nodeForSeat(seat);
      const pav = pavById.get(seat.pavilionId);
      slot = { kind: 'desk', side: seat.side, y: 0, x: seat.x, z: seat.z, rotY: t, chair: at(0, -SEAT_OFF), node, stand: [at(laneX, -SEAT_OFF)],
        corr: walk.node(0, node.ap, []), focus: seat.type === 'cabin' && pav ? { x: pav.x, z: pav.z } : null };
    } else if (seat.kind === 'stool') {
      // an intern stool on the ground: a round slot at level 0, the agent hides its monitor for seatKind stool
      const rig = rigOf(seat);
      const node = walk.nodeForSeat(seat);
      const out = { x: seat.x - Math.sin(t) * 0.5, z: seat.z - Math.cos(t) * 0.5 };
      slot = { kind: 'round', side: 1, y: 0, x: rig.x, z: rig.z, rotY: t, chair: { x: seat.x, z: seat.z }, node, stand: [out],
        corr: walk.node(0, node.ap, []), focus: null };
    } else {
      // review and meeting seats: the agent's rig sits one chair offset toward the table
      const rig = rigOf(seat);
      const node = walk.nodeForSeat(seat);
      const out = { x: seat.x - Math.sin(t) * 0.5, z: seat.z - Math.cos(t) * 0.5 };
      slot = { kind: 'round', side: 1, y: MEZ, x: rig.x, z: rig.z, rotY: t, chair: { x: seat.x, z: seat.z }, node, stand: [out],
        corr: walk.node(1, node.ap, []), focus: null };
    }
    Object.assign(slot, { itemId: seat.id, level, room: seat.pavilionId, seatKind: seat.kind, account: seat.account, reachable: walk.reachable(slot.node) });
    slotOf.set(seat.id, slot);
    return slot;
  }

  // a standing place without furniture: overflow spots, the lobby, anywhere an entry names by position
  const standSlots = new Map();
  function standSlot(level, x, z, rot = 0) {
    const key = `${level}:${x.toFixed(2)}:${z.toFixed(2)}`;
    if (standSlots.has(key)) return standSlots.get(key);
    const p = { x, z };
    const node = walk.node(level, p, []);
    const slot = { kind: 'stand', side: 1, y: level ? MEZ : 0, x, z, rotY: rad(rot), chair: p, node, stand: [p], corr: node, focus: null,
      itemId: null, level, room: null, seatKind: 'stand', reachable: walk.reachable(node) };
    standSlots.set(key, slot);
    return slot;
  }

  // break spots and the groups the break planner draws from
  const spots = {};
  const groups = {};
  for (const s of seats) {
    if (s.kind !== 'cafe' && s.kind !== 'play') continue;
    const spot = { name: s.id, x: s.x, z: s.z, yaw: rad(s.rot), act: s.act, sit: !!s.sit, level: 0, item: null, partner: s.partner, node: walk.node(0, { x: s.x, z: s.z }, []) };
    spots[spot.name] = spot;
    (groups[GROUP_OF[s.act]] = groups[GROUP_OF[s.act]] || []).push(spot);
  }

  // regions for the hover card: the bounding box of each pavilion, the treehouse split into its two rooms
  const regions = [];
  for (const p of layout.pavilions || []) {
    if (p.type === 'treehouse') {
      for (const [id, label, x0, x1] of [['review', 'Review', TH_X - 4.2, TH_X], ['meeting', 'Meeting', TH_X, TH_X + 4.2]]) {
        const bb = boundsOf([[x0, -2.1], [x1, -2.1], [x1, 2.1], [x0, 2.1]].map(([lx, lz]) => toWorld(p, lx, lz)));
        regions.push({ kind: 'room', id: `${p.id}:${id}`, type: id, label, pavilion: p.id, x0: bb.x0, x1: bb.x1, z0: bb.z0, z1: bb.z1, y: MEZ, level: 1 });
      }
      continue;
    }
    const bb = boundsOf(polyOf(p));
    regions.push({ kind: 'room', id: p.id, type: p.type, label: p.label, pavilion: p.id, x0: bb.x0, x1: bb.x1, z0: bb.z0, z1: bb.z1, y: 0, level: 0 });
  }

  return { v: 2, layout, walk, seats, seatById, overflow, W: b.x1 - b.x0, D: b.z1 - b.z0, bounds: b, regions, spots, groups, seatSlot, standSlot,
    spawn: walk.spawn, path: walk.path, node: walk.node, slots: new Map(), key: 'garden:' + (layout.name || '') };
}
