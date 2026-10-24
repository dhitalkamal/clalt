// what stands inside each pavilion type, in the pavilion's local frame (x east, z south, origin at the footprint
// centre). the builders draw it, seats.js turns the spots into seats and walk.js turns the solids into walls,
// so all three read one table and cannot drift apart.
import { sizeOf, deskCells, DESK_TYPES } from './footprints.js';

export const MEZ = 1.29; // deck height of the treehouse
export const TH_X = -0.55; // the treehouse platform is 8.4 wide, shifted so the stair fits the 9.6 footprint
export const SEAT_OFF = 0.36; // chair centre behind the desk centre

export const CAFE_TABLES = [[-1.3, 0.7], [1.5, 0.8]];
export const BEANBAGS = [[-1.9, 0.6], [-0.7, 1.2], [-1.6, 1.55]];
export const PONG_TABLE = [1.2, -0.5];
// the east side table stands clear of the stair exit at the north east corner
export const TH_SIDE_TABLES = [[TH_X - 3.7, -1.8], [TH_X + 3.7, -0.9]];
export const TH_TABLES = { review: [TH_X - 2.1, 0.25], meeting: [TH_X + 2.1, 0.3] };

// stairs of the treehouse: foot on the ground at the south end, top on the deck level at the north end, rising north
export const TH_STAIR = { x: 4.25, foot: 1.9, top: -1.6, landing: -2.1, w: 1.0 };

// a break spot or a seat: {key, kind, act, sit, lx, lz, face:[lx,lz], partner?}
const spot = (key, kind, act, sit, lx, lz, face, partner) => ({ key, kind, act, sit, lx, lz, face, partner });

export function cafeSpots() {
  const out = [spot('order-a', 'cafe', 'order', false, -1.1, -0.7, [-1.1, -1.3]), spot('order-b', 'cafe', 'order', false, 0.1, -0.7, [0.1, -1.3])];
  CAFE_TABLES.forEach(([tx, tz], i) => {
    out.push(spot(`sip-${i}a`, 'cafe', 'sip', true, tx - 0.5, tz, [tx, tz], `sip-${i}b`), spot(`sip-${i}b`, 'cafe', 'sip', true, tx + 0.5, tz, [tx, tz], `sip-${i}a`));
  });
  return out;
}

export function playSpots() {
  const [px, pz] = PONG_TABLE;
  const out = [spot('pong-a', 'play', 'pong', false, px - 1.0, pz, [px, pz], 'pong-b'), spot('pong-b', 'play', 'pong', false, px + 1.0, pz, [px, pz], 'pong-a')];
  BEANBAGS.forEach(([x, z], i) => out.push(spot(`bag-${i}`, 'play', 'read', true, x, z, [-1.2, 0.9])));
  return out;
}

// arrivals wait around the fountain. they face south, toward the default camera
export function lobbySpots() {
  return [[-0.1, -1.1], [0.8, -1.1], [-1.4, -1.1], [-0.2, 0.6], [-1.2, 1.2]].map(([x, z], i) => spot(`wait-${i}`, 'lobby', 'stand', false, x, z, [x, z + 1]));
}

// three stools round each treehouse table. y is the deck height
export function treehouseSeats() {
  const out = [];
  for (const kind of ['review', 'meeting']) {
    const [tx, tz] = TH_TABLES[kind];
    // chairs sit 1.1 from the table centre so the monitors of three agents stay apart
    [[-1.1, 0], [1.1, 0], [0, 1.1]].forEach(([dx, dz], i) => out.push(spot(`${kind}-${i}`, kind, 'sit', true, tx + dx, tz + dz, [tx, tz])));
  }
  return out;
}

export const lobbyGate = (p) => {
  const d = sizeOf(p).d;
  return { arch: [0, -d / 2 + 0.2], inside: [0, -d / 2 + 0.9] };
};

const rect = (x0, z0, x1, z1, wall = false) => ({ x0, z0, x1, z1, wall });
const disc = (x, z, r) => ({ x, z, r });

// solids for the walk grid, per level: rects (x0,z0,x1,z1, wall flag inflates thin ones) and discs
export function solidsOf(p) {
  const s = sizeOf(p);
  const hw = s.w / 2;
  const hd = s.d / 2;
  const rects = [];
  const discs = [];
  if (DESK_TYPES.has(p.type)) {
    // a desk spans -0.3 to +0.4 in front of its centre, the south line of a paired row is turned round
    for (const c of deskCells(p)) rects.push(c.line ? rect(c.lx - 0.7, c.lz - 0.4, c.lx + 0.7, c.lz + 0.3) : rect(c.lx - 0.7, c.lz - 0.3, c.lx + 0.7, c.lz + 0.4));
    if (p.type === 'cabin') {
      rects.push(rect(-hw, -hd, hw, -hd + 0.25, true), rect(-hw, -hd, -hw + 0.1, hd, true), rect(hw - 0.1, -hd, hw, hd, true));
      // the door gap is wider than the pickets show, so the grid always keeps a cell open in it
      rects.push(rect(-hw, hd - 0.1, -0.5, hd, true), rect(0.5, hd - 0.1, hw, hd, true));
      discs.push(disc(hw - 0.35, hd - 0.35, 0.2));
    } else {
      for (const x of [-hw + 0.04, hw - 0.04]) for (const z of [-hd + 0.04, hd - 0.04]) discs.push(disc(x, z, 0.12));
      // bookcases line the back with a doorway in the middle
      if (p.type === 'research') rects.push(rect(-hw, -hd, -0.5, -hd + 0.35, true), rect(0.5, -hd, hw, -hd + 0.35, true));
      discs.push(disc(hw - 0.3, hd - 0.3, 0.2));
    }
  } else if (p.type === 'lobby') {
    discs.push(disc(-0.95, -0.1, 0.4), disc(-1.6, -hd + 0.3, 0.1), disc(1.6, -hd + 0.3, 0.1), disc(-1.55, hd - 0.3, 0.2));
    rects.push(rect(0.45, 0.45, 1.55, 0.95));
  } else if (p.type === 'cafe') {
    rects.push(rect(-1.8, -1.55, 0.8, -1.05));
    for (const [x, z] of CAFE_TABLES) discs.push(disc(x, z, 0.32));
    discs.push(disc(hw - 0.3, -hd + 0.3, 0.1), disc(-hw + 0.3, hd - 0.4, 0.1), disc(hw - 0.35, hd - 0.35, 0.2));
  } else if (p.type === 'play') {
    rects.push(rect(PONG_TABLE[0] - 0.8, PONG_TABLE[1] - 0.45, PONG_TABLE[0] + 0.8, PONG_TABLE[1] + 0.45));
  }
  return { rects, discs };
}

// treehouse deck level: floors, solids and the rope rails, in the footprint frame
export function deckLevel() {
  const x0 = TH_X - 4.2;
  const x1 = TH_X + 4.2;
  const floors = [rect(x0, -2.1, x1, 2.1), rect(x1 - 0.05, TH_STAIR.landing - 0.5, TH_STAIR.x + 0.55, TH_STAIR.landing + 0.5)];
  const rects = [rect(x0, 1.95, x1, 2.1, true), rect(x0, -2.1, x0 + 0.1, 2.1, true), rect(x1 - 0.1, 0.2, x1, 2.1, true), rect(TH_X + 2.1 - 0.65, -1.6, TH_X + 2.1 + 0.65, -1.4)];
  const discs = [disc(...TH_TABLES.review, 0.75), disc(...TH_TABLES.meeting, 0.75), ...TH_SIDE_TABLES.map(([x, z]) => disc(x, z, 0.3)), disc(TH_X - 0.5, -1.8, 0.25)];
  return { floors, rects, discs };
}

// ground level of the treehouse: only the stair foot is open, the platform is above it
export function groundLevel() {
  return { floors: [rect(TH_STAIR.x - 0.5, TH_STAIR.foot + 0.05, TH_STAIR.x + 0.5, TH_STAIR.foot + 0.8)], rects: [], discs: [] };
}
