// v6 floor plan on a 0.5 unit grid, built for one fixed front camera (looking toward -z). all local
// coordinates are multiples of G, X() and Z() shift them into world space (W and D are whole numbers).
//   back band : glass cabins in account wings (2.0 x 2.0), the glass front faces the camera
//   middle    : lobby | research | open workspace, desks in columns, characters face the camera
//   annex     : mezzanine deck (review and meeting rooms) with a staircase, cafe and play area in front of it
export const G = 0.5;
export const CW = 2.0;
export const CD = 2.0;
export const CROW = 3.0; // cabin row pitch
export const MEZ = 1.29; // deck height, matches the wall tile seam
export const SEAT_OFF = 0.36; // chair centre behind the desk centre (north of it)
export const COL = 2.0; // desk column pitch: chair, desk, side return and a walking lane
export const ROW = 1.5; // desk row pitch
// wing order, set from the snapshot so accounts keep their place while sessions come and go
export const ACCOUNT_ORDER = [];
export function setAccountOrder(ids) { ACCOUNT_ORDER.length = 0; ACCOUNT_ORDER.push(...ids); }
export const SPACES = [{ id: 'research', label: 'Research', minCols: 2 }, { id: 'open', label: 'Open workspace', minCols: 2 }];
const RING = [0, 45, -45, 90, -90, 135, -135, 180].map((d) => (d * Math.PI) / 180);
export const RING_R = 0.85;

export function spaceFor(type) {
  if (type === 'Explore') return 'research';
  if (type === 'Plan') return 'meeting';
  if (/review|test|hunter|analy/i.test(type)) return 'review';
  return 'open';
}

const pt = (x, z, y = 0) => ({ x, z, y });

export function computeLayout(sessions) {
  const accounts = ACCOUNT_ORDER.filter((a) => sessions.some((s) => s.account === a));
  for (const s of sessions) if (!accounts.includes(s.account)) accounts.push(s.account);

  // cabin wings
  let x = 1.0;
  let cabinRows = 1;
  const wings = accounts.map((account) => {
    const list = sessions.filter((s) => s.account === account);
    const cols = Math.min(3, list.length);
    const rows = Math.ceil(list.length / cols);
    cabinRows = Math.max(cabinRows, rows);
    const w = { account, x0: x, x1: x + cols * CW, cols, rows, list };
    x = w.x1 + 0.5;
    return w;
  });
  const cabinRight = accounts.length ? x - 0.5 : 1.0;
  const cabinEnd = 0.5 + CD + (cabinRows - 1) * CROW;
  const lane0z = cabinEnd + 0.5;
  const spacesZ = cabinEnd + 1.0;
  const SP_D = 5.0;

  // desk columns of three
  const counts = {};
  for (const s of sessions) for (const sub of s.subs) counts[sub.space] = Math.max(counts[sub.space] || 0, sub.seatIndex + 1);
  let sx = 2.0;
  const spaces = SPACES.map((def) => {
    const n = counts[def.id] || 0;
    const cols = Math.max(def.minCols, Math.ceil(n / 3));
    const sp = { ...def, n, cols, x0: sx, x1: sx + cols * COL + 0.5, z0: spacesZ, z1: spacesZ + SP_D };
    sx = sp.x1 + 0.5;
    return sp;
  });
  const spacesRight = sx - 0.5;

  // annex: deck 8 wide at the back, stairs on its east side, cafe and play area in front of it
  const ax0 = Math.max(cabinRight, spacesRight) + 1.0;
  const deckW = 8.0;
  const dz0 = 0.5;
  const dz1 = 4.5;
  const stairX0 = ax0 + deckW;
  const cz0 = 5.0;
  const frontD = 3.0;
  const W = Math.ceil(stairX0 + 1.5);
  const D = Math.ceil(Math.max(spacesZ + SP_D + 1.0, cz0 + frontD + 1.0));
  const ox = -W / 2;
  const oz = -D / 2;
  const X = (v) => ox + v;
  const Z = (v) => oz + v;
  const door = { z: Z(D - 0.5), aisleX: X(0.5), outX: X(-0.7) };
  const lane0 = Z(lane0z);
  const FZ = Z(cz0 + frontD - 0.25);
  const laneX = X(ax0 - 0.5);

  const base = [pt(door.outX, door.z), pt(door.aisleX, door.z), pt(door.aisleX, lane0)];
  const same = (p, q) => Math.abs(p.x - q.x) < 1e-6 && Math.abs(p.z - q.z) < 1e-6 && Math.abs(p.y - q.y) < 1e-6;
  const path = (a, b) => {
    const ra = a.route;
    const rb = b.route;
    let k = 0;
    while (k < ra.length - 1 && k < rb.length - 1 && same(ra[k + 1], rb[k + 1]) && same(ra[k], rb[k])) k++;
    return [...ra.slice(k).reverse(), ...rb.slice(k + 1)];
  };
  const spawn = { route: [pt(door.outX, door.z)] };
  const toFront = [pt(laneX, lane0), pt(laneX, FZ)];
  const stairX = X(stairX0 + 0.5);
  const stairZb = Z(dz1 + 3.0);
  const toStairs = [...toFront, pt(stairX, FZ), pt(stairX, stairZb), pt(stairX, Z(dz1), MEZ)];

  const slots = new Map();
  const regions = [];
  // side: which side of the desk the monitor return sits on (+1 east, -1 west)
  const deskSlot = (xw, zw, y, route, corr, side, extra = {}) => {
    const chair = { x: xw, z: zw - SEAT_OFF };
    return { y, x: xw, z: zw, rotY: 0, side, chair, kind: 'desk', stand: null, node: { route: [...route, chair] }, corr: { route: corr }, ...extra };
  };

  for (const w of wings) {
    w.list.forEach((s, i) => {
      const col = i % w.cols;
      const row = Math.floor(i / w.cols);
      const cx = X(w.x0 + (col + 0.5) * CW);
      const cz = Z(0.5 + CD / 2 + row * CROW);
      const doorX = cx + 0.8;
      const front = cz + CD / 2;
      const lane = row === 0 ? lane0 : front + 0.5;
      const rt = row === 0 ? base : [...base, pt(door.aisleX, lane)];
      const deskX = cx + 0.25;
      const deskZ = cz + 0.25;
      const chairZ = deskZ - SEAT_OFF;
      const slot = deskSlot(deskX, deskZ, 0, [...rt, pt(doorX, lane), pt(doorX, chairZ)], [...rt, pt(doorX, lane)], -1);
      slot.stand = [{ x: doorX, z: chairZ }, { x: doorX, z: front + 0.3 }];
      slots.set(s.id, slot);
      s.cabin = { cx, cz, doorX, front, back: cz - CD / 2 };
      regions.push({ kind: 'cabin', id: s.id, x0: cx - CW / 2, x1: cx + CW / 2, z0: cz - CD / 2, z1: front + 0.6 });
    });
    w.cx = X((w.x0 + w.x1) / 2);
    w.z0 = Z(0.5);
    w.z1 = Z(0.5 + CD + (w.rows - 1) * CROW);
    w.wx0 = X(w.x0);
    w.wx1 = X(w.x1);
    regions.push({ kind: 'wing', id: w.account, x0: w.wx0, x1: w.wx1, z0: w.z0, z1: w.z1 + 0.6 });
  }
  for (const sp of spaces) {
    sp.wx0 = X(sp.x0); sp.wx1 = X(sp.x1); sp.wz0 = Z(sp.z0); sp.wz1 = Z(sp.z1);
    sp.cx = (sp.wx0 + sp.wx1) / 2;
    sp.rows = 3;
    regions.push({ kind: 'space', id: sp.id, x0: sp.wx0, x1: sp.wx1, z0: sp.wz0, z1: sp.wz1 });
  }
  const spaceById = Object.fromEntries(spaces.map((s) => [s.id, s]));

  // mezzanine rooms, 4 wide each
  const review = { id: 'review', label: 'Review room', y: MEZ, wx0: X(ax0), wx1: X(ax0 + 4), wz0: Z(dz0), wz1: Z(dz1) };
  const meeting = { id: 'meeting', label: 'Meeting room', y: MEZ, wx0: X(ax0 + 4), wx1: X(ax0 + 8), wz0: Z(dz0), wz1: Z(dz1) };
  for (const r of [review, meeting]) {
    r.cx = (r.wx0 + r.wx1) / 2; r.n = counts[r.id] || 0;
    regions.push({ kind: 'space', id: r.id, x0: r.wx0, x1: r.wx1, z0: r.wz0, z1: r.wz1, y: MEZ });
  }
  review.table = { x: X(ax0 + 2), z: Z(2.5) };
  const deck = { x0: X(ax0), x1: X(ax0 + deckW), z0: Z(dz0), z1: Z(dz1), y: MEZ, stairX0: X(stairX0), stairW: 1.0, zb: stairZb, zt: Z(dz1) };
  const deckLead = Z(dz1 - 0.5);
  const toDeck = [...toStairs, pt(stairX, deckLead, MEZ)];

  for (const sess of sessions) {
    for (const sub of sess.subs) {
      if (sub.space === 'review') {
        const a = RING[sub.seatIndex % 8];
        const T = review.table;
        const seat = { x: T.x + Math.sin(a) * RING_R, z: T.z - Math.cos(a) * RING_R };
        const rotY = Math.atan2(T.x - seat.x, T.z - seat.z);
        const rig = { x: seat.x + Math.sin(rotY) * SEAT_OFF, z: seat.z + Math.cos(rotY) * SEAT_OFF };
        const appr = { x: T.x + (seat.x - T.x) * 1.7, z: T.z + (seat.z - T.z) * 1.7 };
        slots.set(sub.id, {
          y: MEZ, x: rig.x, z: rig.z, rotY, side: 1, chair: seat, stand: null, kind: 'round',
          node: { route: [...base, ...toDeck, pt(appr.x, deckLead, MEZ), pt(appr.x, appr.z, MEZ), pt(seat.x, seat.z, MEZ)] },
          corr: { route: [...base, ...toDeck] },
        });
      } else if (sub.space === 'meeting') {
        const col = sub.seatIndex % 2;
        const row = Math.floor(sub.seatIndex / 2) % 2;
        const dx = X(ax0 + 4 + 1.0 + col * COL);
        const dz = Z(dz0 + 1.5 + row * ROW);
        const lx = dx - 0.8;
        slots.set(sub.id, deskSlot(dx, dz, MEZ, [...base, ...toDeck, pt(lx, deckLead, MEZ), pt(lx, dz - SEAT_OFF, MEZ)], [...base, ...toDeck], 1));
      } else {
        const sp = spaceById[sub.space];
        const col = Math.floor(sub.seatIndex / 3);
        const row = sub.seatIndex % 3;
        const dx = sp.wx0 + 1.5 + col * COL;
        const dz = sp.wz0 + 1.0 + row * ROW;
        const lx = dx - 0.8;
        slots.set(sub.id, deskSlot(dx, dz, 0, [...base, pt(lx, lane0), pt(lx, dz - SEAT_OFF)], [...base, pt(lx, lane0)], 1));
      }
    }
  }

  // ground floor: cafe and play area, 4 wide each
  const cafe = { id: 'cafe', label: 'Cafe', wx0: X(ax0), wx1: X(ax0 + 4), wz0: Z(cz0), wz1: Z(cz0 + frontD) };
  const rec = { id: 'rec', label: 'Play area', wx0: X(ax0 + 4), wx1: X(ax0 + 8), wz0: Z(cz0), wz1: Z(cz0 + frontD) };
  for (const r of [cafe, rec]) { r.cx = (r.wx0 + r.wx1) / 2; regions.push({ kind: 'space', id: r.id, x0: r.wx0, x1: r.wx1, z0: r.wz0, z1: r.wz1 }); }
  const lobby = { wx0: X(0), wx1: X(2), wz0: Z(spacesZ), wz1: Z(D) };
  regions.push({ kind: 'lobby', id: 'lobby', x0: lobby.wx0, x1: lobby.wx1, z0: lobby.wz0, z1: lobby.wz1 });

  // named leisure spots. yaw: 0 faces the camera, small turns point toward the thing they use
  const spots = {};
  const spot = (name, x, z, yaw, act, extra = {}) => {
    spots[name] = { name, x, z, yaw, act, node: { route: extra.route || [...base, ...toFront, pt(x, FZ), pt(x, z)] }, ...extra };
  };
  const cx0 = cafe.wx0;
  const cz = cafe.wz0;
  const rx0 = rec.wx0;
  spot('counterA', cx0 + 1.25, cz + 1.0, -0.35, 'order');
  spot('counterB', cx0 + 2.25, cz + 1.0, -0.35, 'order');
  spot('tableW', cx0 + 0.5, cz + 2.0, 0.9, 'sip', { partner: 'tableE' });
  spot('tableE', cx0 + 1.5, cz + 2.0, -0.9, 'sip', { partner: 'tableW' });
  spot('tableN', cx0 + 2.5, cz + 2.0, 0.9, 'sip', { partner: 'tableS' });
  spot('tableS', cx0 + 3.5, cz + 2.0, -0.9, 'sip', { partner: 'tableN' });
  spot('pongA', rx0 + 1.0, cz + 2.0, Math.PI / 2, 'pong', { partner: 'pongB' });
  spot('pongB', rx0 + 3.0, cz + 2.0, -Math.PI / 2, 'pong', { partner: 'pongA' });
  spot('arcade1', rx0 + 2.75, cz + 0.5, 1.0, 'arcade');
  spot('arcade2', rx0 + 2.75, cz + 1.0, 1.0, 'arcade');
  spot('bagA', rx0 + 0.5, cz + 0.5, 0.3, 'read', { sit: true });
  spot('bagB', rx0 + 1.0, cz + 1.25, -0.3, 'read', { sit: true });
  const wz = Z(4.5);
  spot('cooler', X(1.0), Z(3.5), 1.0, 'cooler', { route: [...base, pt(door.aisleX, Z(3.5)), pt(X(1.0), Z(3.5))] });
  spot('window', X(1.0), wz, -0.9, 'window', { route: [...base, pt(door.aisleX, wz), pt(X(1.0), wz)] });

  const P = {
    counter: { x: cx0, z: cz }, table1: { x: cx0 + 1.0, z: cz + 2.0 }, table2: { x: cx0 + 3.0, z: cz + 2.0 },
    pong: { x: rx0 + 2.0, z: cz + 2.0 }, bags: [{ x: rx0 + 0.5, z: cz + 0.5 }, { x: rx0 + 1.0, z: cz + 1.25 }],
    arcades: [{ x: rx0 + 3.5, z: cz + 0.5 }, { x: rx0 + 3.5, z: cz + 1.0 }],
  };
  return { W, D, G, xmin: X(0), zmin: Z(0), X, Z, wings, spaces, lobby, review, meeting, cafe, rec, deck, P, slots, regions, spots,
    door, lane0, laneX, FZ, spacesZ: Z(spacesZ), spacesBottom: Z(spacesZ + SP_D), cabinEnd: Z(cabinEnd), path, spawn, annexX0: X(ax0),
    stairX, deckLead, key: `${W}x${D}` };
}
