// signatures: planSeats(layout, roster, catalog?, prev?) and breakSpots(layout, catalog?).
// catalog is [{id, footprint, tags}]; without it each furniture item's own tags and footprint are
// used and one console.warn is logged per layout object.
// seat planning for the office page: which agent sits where, and where idle agents take a break.
// pure functions over a layout json (v1), no dom, no three.js.
import { catalogIndex, spotsWithTags, blockedCells, freeCells, doorCell } from './routing/spatial.js';

const warned = new WeakSet();
function checkCatalog(catalog, layout) {
  if (catalog?.length || warned.has(layout)) return;
  warned.add(layout);
  console.warn('routing: no catalog given, using the tags on each furniture item');
}

// room type a subagent type is routed to; anything unknown goes to the workspace
export function roomTypeFor(type) {
  const t = String(type || '').toLowerCase();
  if (t === 'explore') return 'research';
  if (t === 'plan') return 'meeting';
  if (/review|test|hunter|analy/.test(t)) return 'review';
  return 'workspace';
}

// can this agent use a room of this layout: its own room kind, or the workspace as overflow
function usable(agent, room) {
  if (room.type === 'workspace') return true;
  if (agent.kind === 'main') return room.type === 'cabins' && (!room.account || room.account === agent.account);
  return room.type === roomTypeFor(agent.type);
}

// seats in preference order for one agent: primary rooms first (cabins restricted to the
// agent's account before open ones), then workspace seats
function chainFor(agent, seats) {
  const primary = agent.kind === 'main' ? 'cabins' : roomTypeFor(agent.type);
  const own = seats.filter((s) => s.room.type === primary && usable(agent, s.room));
  if (primary === 'cabins') own.sort((a, b) => (b.room.account === agent.account ? 1 : 0) - (a.room.account === agent.account ? 1 : 0));
  if (primary === 'workspace') return own;
  return own.concat(seats.filter((s) => s.room.type === 'workspace'));
}

const planned = (s) => ({ kind: 'seat', level: s.level, x: s.x, z: s.z, rot: s.rot, room: s.room.id, seat: s.id });
const standing = (c) => ({ kind: 'stand', level: c.level, x: c.x, z: c.z, rot: c.rot, room: c.room });
const byKey = (a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0);

// planSeats(layout, roster, catalog?, prev?) -> Map(agentKey -> {kind, level, x, z, rot, room, seat?})
// roster = [{key, kind: "main"|"sub", account, type, state}]. x and z are cell coordinates of the
// seat center (or the standing cell center), rot is the furniture rotation in degrees.
// prev is the last returned plan: agents keep a still valid seat or standing cell so nobody
// shuffles between polls. mains are placed before subs, each group in key order, so the result
// does not depend on roster order.
export function planSeats(layout, roster, catalog, prev) {
  checkCatalog(catalog, layout);
  const by = catalogIndex(catalog);
  const seats = spotsWithTags(layout, by, ['seat']);
  const seatById = new Map(seats.map((s) => [s.id, s]));
  const claimed = new Set();
  const plan = new Map();
  const first = new Map();
  for (const a of roster) if (!first.has(a.key)) first.set(a.key, a);
  const agents = [...first.values()].sort((a, b) => (a.kind === b.kind ? byKey(a, b) : a.kind === 'main' ? -1 : 1));

  // keep previous seats that still exist and are still allowed
  for (const a of agents) {
    const old = prev?.get(a.key);
    const s = old?.kind === 'seat' ? seatById.get(old.seat) : null;
    if (s && !claimed.has(s.id) && usable(a, s.room)) { claimed.add(s.id); plan.set(a.key, planned(s)); }
  }

  // everyone else takes the first free seat in their chain
  const chains = new Map();
  const loose = [];
  for (const a of agents) {
    if (plan.has(a.key)) continue;
    const ck = a.kind === 'main' ? 'main:' + a.account : 'sub:' + roomTypeFor(a.type);
    if (!chains.has(ck)) chains.set(ck, { list: chainFor(a, seats), at: 0 });
    const c = chains.get(ck);
    while (c.at < c.list.length && claimed.has(c.list[c.at].id)) c.at++;
    if (c.at < c.list.length) { const s = c.list[c.at]; claimed.add(s.id); plan.set(a.key, planned(s)); } else loose.push(a);
  }

  placeStanding(layout, by, loose, prev, plan);
  return plan;
}

// lobby floor cells for agents without a seat, falling back to the door
function placeStanding(layout, by, loose, prev, plan) {
  if (!loose.length) return;
  const lobby = (layout.rooms || []).find((r) => r.type === 'lobby');
  const cells = lobby ? freeCells(lobby, blockedCells(layout, by)) : [];
  const key = (c) => c.level + ',' + c.x + ',' + c.z;
  const valid = new Map(cells.map((c) => [key(c), c]));
  const taken = new Set();
  const rest = [];
  for (const a of loose) {
    const old = prev?.get(a.key);
    const c = old?.kind === 'stand' ? valid.get(key(old)) : null;
    if (c && !taken.has(key(c))) { taken.add(key(c)); plan.set(a.key, standing(c)); } else rest.push(a);
  }
  let at = 0;
  let extra = 0;
  for (const a of rest) {
    while (at < cells.length && taken.has(key(cells[at]))) at++;
    let c;
    if (at < cells.length) c = cells[at++];
    else if (cells.length) c = cells[extra++ % cells.length]; // more agents than cells: reuse them
    else c = doorCell(layout);
    plan.set(a.key, standing(c));
  }
}

// breakSpots(layout, catalog?) -> {cafe, play, all}. each spot is {kind, level, x, z, rot, room, seat?}.
// spots are tagged furniture (seat, play, break) in cafe and play rooms; a room with none offers
// a few free floor cells; with neither room type the lobby floor is used so idle agents still have
// somewhere to stand.
export function breakSpots(layout, catalog) {
  checkCatalog(catalog, layout);
  const by = catalogIndex(catalog);
  const tagged = spotsWithTags(layout, by, ['seat', 'play', 'break']);
  const blocked = blockedCells(layout, by);
  const forType = (type) => {
    const out = [];
    for (const r of (layout.rooms || []).filter((x) => x.type === type)) {
      const mine = tagged.filter((s) => s.room.id === r.id);
      if (mine.length) out.push(...mine.map((s) => ({ kind: type, level: s.level, x: s.x, z: s.z, rot: s.rot, room: r.id, seat: s.id })));
      else out.push(...freeCells(r, blocked, 8).map((c) => ({ kind: type, ...c })));
    }
    return out;
  };
  const cafe = forType('cafe');
  const play = forType('play');
  let all = cafe.concat(play);
  if (!all.length) {
    const lobby = (layout.rooms || []).find((r) => r.type === 'lobby');
    if (lobby) all = freeCells(lobby, blocked, 12).map((c) => ({ kind: 'lobby', ...c }));
  }
  return { cafe, play, all };
}
