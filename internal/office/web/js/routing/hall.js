// seat planning for v1 layouts (the hall): working agents get desks first, resting ones take the rest and
// otherwise wait in the cafe, play area or lobby. pure, no dom.
import { RANK, placeHallInterns } from './team.js';
import { allocDesks } from './alloc.js';
import { spotsWithTags, blockedCells, freeCells, doorCell } from './spatial.js';

// room type a subagent type is routed to; anything unknown goes to the workspace
export function roomTypeFor(type) {
  const t = String(type || '').toLowerCase();
  if (t === 'explore') return 'research';
  if (t === 'plan') return 'meeting';
  if (/review|test|hunter|analy/.test(t)) return 'review';
  return 'workspace';
}

const WORK = ['workspace', 'research'];
const planned = (s) => ({ kind: 'seat', level: s.level, x: s.x, z: s.z, rot: s.rot, room: s.room.id, seat: s.id });
const standing = (c) => ({ kind: 'stand', level: c.level, x: c.x, z: c.z, rot: c.rot, room: c.room });
const byKey = (a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0);
const idle = (a) => a.state === 'idle';
// distance between seats, a level change counts as far
const dist = (a, b) => Math.hypot(a.x - b.x, a.z - b.z) + Math.abs(a.level - b.level) * 100;

// working agents first (sessions, members, interns, subs), then resting members, then other resting agents
function priority(a) {
  if (idle(a)) return a.kind === 'team' ? 10 : 11;
  return RANK[a.kind] ?? 2;
}

// seats an agent may use in preference order. interns sit nearest to their member, anywhere on a working desk
function poolFor(a, seats, got) {
  const on = (t) => seats.filter((s) => s.room.type === t);
  if (a.kind === 'team') return WORK.flatMap(on);
  if (a.kind === 'intern') {
    const list = WORK.flatMap(on);
    const m = got.get('team:' + a.team);
    return m ? list.map((s) => [dist(m, s), s]).sort((p, q) => p[0] - q[0]).map((p) => p[1]) : list;
  }
  if (a.kind === 'main') {
    const own = on('cabins').filter((s) => !s.room.account || s.room.account === a.account);
    own.sort((p, q) => (q.room.account === a.account ? 1 : 0) - (p.room.account === a.account ? 1 : 0));
    return own.concat(on('workspace'));
  }
  const primary = roomTypeFor(a.type);
  return primary === 'workspace' ? on('workspace') : on(primary).concat(on('workspace'));
}

// planSeats for v1. mains, then members, interns and subs; the roster order of members is kept
export function planHall(layout, by, roster, prev) {
  const seats = spotsWithTags(layout, by, ['seat']);
  const first = new Map();
  for (const a of roster) if (!first.has(a.key)) first.set(a.key, a);
  const order = new Map(roster.map((a, i) => [a.key, i]));
  const agents = [...first.values()].sort((a, b) => (priority(a) - priority(b))
    || (a.kind === 'team' && b.kind === 'team' ? order.get(a.key) - order.get(b.key) : byKey(a, b)));
  const { got } = allocDesks({ agents, prev, seats, idOf: (s) => s.id, pool: (a, g) => poolFor(a, seats, g), idle, evictable: (a) => a.kind !== 'main' });
  const plan = new Map();
  for (const [key, s] of got) plan.set(key, planned(s));

  // no desk left: interns stand beside their member, the rest wait (resting ones in the cafe first)
  const interns = agents.filter((a) => a.kind === 'intern' && !plan.has(a.key));
  const loose = placeHallInterns(layout, by, interns, plan, prev);
  const rest = agents.filter((a) => a.kind !== 'intern' && !plan.has(a.key));
  const taken = new Set();
  placeStanding(layout, by, loose.concat(rest.filter((a) => !idle(a))), prev, plan, ['lobby'], taken);
  placeStanding(layout, by, rest.filter(idle), prev, plan, ['cafe', 'play', 'lobby'], taken);
  return plan;
}

// floor cells of the given room types for agents without a seat, falling back to the door
function placeStanding(layout, by, loose, prev, plan, types, taken) {
  if (!loose.length) return;
  const blocked = blockedCells(layout, by);
  const cells = types.flatMap((t) => (layout.rooms || []).filter((r) => r.type === t).flatMap((r) => freeCells(r, blocked)));
  const key = (c) => c.level + ',' + c.x + ',' + c.z;
  const valid = new Map(cells.map((c) => [key(c), c]));
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
    if (at < cells.length) { c = cells[at++]; taken.add(key(c)); } else if (cells.length) c = cells[extra++ % cells.length]; // more agents than cells: reuse them
    else c = doorCell(layout);
    plan.set(a.key, standing(c));
  }
}
