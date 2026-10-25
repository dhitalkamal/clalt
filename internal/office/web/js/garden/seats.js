// seats and overflow spots of a v2 layout, in world coordinates. pure, no three.js.
// gardenSeats(layout) -> [{id, pavilionId, type, account, y, x, z, rot, kind, ...}]. rot is degrees, the facing of
// the seat (the pavilion's rot for desks). a desk seat is the desk centre: the chair is SEAT_OFF behind it (north
// at rot 0). round seats (review, meeting) are the chair centre and face the table. extra fields: act, sit, face
// (world point to look at), partner (seat id), lane (+1 east or -1 west, desks), col and row (desks).
import { deskCells, toWorld, DESK_TYPES } from './footprints.js';
import { MEZ, cafeSpots, playSpots, lobbySpots, treehouseSeats } from './local.js';

export { MEZ };

const deg = (v) => Math.round(v * 1e6) / 1e6;

// rotation in degrees that faces a world point from a seat, 0 faces +z
export const facing = (x, z, fx, fz) => deg((Math.atan2(fx - x, fz - z) * 180) / Math.PI);

function deskSeats(p) {
  const team = p.type === 'team';
  const members = team ? (Array.isArray(p.members) ? p.members : []) : null;
  const out = [];
  deskCells(p).forEach((c, i) => {
    // a team desk exists only for a member: members[i] sits at the i-th cell in desk order
    if (team && i >= members.length) return;
    const [x, z] = toWorld(p, c.lx, c.lz);
    // the lane to the chair runs between two desks: east in the world for even columns, west for odd. the south
    // line of a paired row is turned round, so its local east is the world's west
    const lane = (c.col % 2 === 0 ? 1 : -1) * (c.line ? -1 : 1);
    const tag = c.line ? `${c.row}b` : `${c.row}${p.type === 'cabin' ? '' : 'a'}`;
    const desk = { id: `${p.id}:desk-${p.type === 'cabin' ? c.row : tag}-${c.col}`, pavilionId: p.id, type: p.type, account: p.type === 'cabin' ? p.account : null,
      y: 0, x, z, rot: (p.rot || 0) + (c.line ? 180 : 0), kind: 'desk', col: c.col, row: c.row, cols: c.cols, line: c.line, lane, side: lane };
    if (team) desk.member = members[i];
    out.push(desk);
    if (team) out.push(...stoolSeats(p, c, desk));
  });
  return out;
}

// three intern stools on the outer side of a team desk (away from the shared lane), in a short column facing
// the desk. the outer side is -dir in the pavilion frame, dir being the pavilion-local lane direction
const STOOL_X = 0.8;
const STOOL_DZ = [-0.45, 0, 0.45];

function stoolSeats(p, c, desk) {
  const dir = c.col % 2 === 0 ? 1 : -1;
  return STOOL_DZ.map((dz, index) => {
    const [x, z] = toWorld(p, c.lx - dir * STOOL_X, c.lz + dz);
    return { id: `${desk.id}:stool-${index}`, pavilionId: p.id, type: p.type, account: null, y: 0, x, z, rot: facing(x, z, desk.x, desk.z),
      kind: 'stool', member: desk.member, index, desk: desk.id };
  });
}

function spotSeats(p, list, y) {
  return list.map((s) => {
    const [x, z] = toWorld(p, s.lx, s.lz);
    const [fx, fz] = toWorld(p, s.face[0], s.face[1]);
    return { id: `${p.id}:${s.key}`, pavilionId: p.id, type: p.type, account: null, y, x, z, rot: facing(x, z, fx, fz), kind: s.kind,
      act: s.act, sit: s.sit, face: [fx, fz], partner: s.partner ? `${p.id}:${s.partner}` : undefined };
  });
}

export function gardenSeats(layout) {
  const out = [];
  for (const p of layout.pavilions || []) {
    if (DESK_TYPES.has(p.type)) out.push(...deskSeats(p));
    else if (p.type === 'cafe') out.push(...spotSeats(p, cafeSpots(), 0));
    else if (p.type === 'play') out.push(...spotSeats(p, playSpots(), 0));
    else if (p.type === 'lobby') out.push(...spotSeats(p, lobbySpots(), 0));
    else if (p.type === 'treehouse') out.push(...spotSeats(p, treehouseSeats(), MEZ));
  }
  return out;
}

// standing spots the generator placed for sessions that find no desk
export function gardenOverflow(layout) {
  return (layout.overflow || []).map((o, i) => ({ id: `overflow-${i}`, x: o.x, z: o.z, y: 0, rot: o.rot || 0, near: o.near }));
}
