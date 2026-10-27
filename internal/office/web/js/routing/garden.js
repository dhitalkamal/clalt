// seat planning for layout v2 (garden campus). pure functions over the seat list the renderer builds,
// no dom, no three.js. coordinates are world units, entries carry world: true so callers do not
// treat them as grid cells.
import { RANK } from './team.js';
import { allocDesks } from './alloc.js';

// where the seat list comes from. the page registers gardenSeats here at boot; tests pass a source in.
let source = null;
export function setGardenSource(fn) { source = fn; }

const f2 = (n) => (Math.round(n * 100) / 100).toFixed(2);
// the renderer's seat id names a seat; one without an id is named by pavilion, kind and place
const seatKey = (s) => s.id || [s.pavilionId, s.kind, f2(s.x), f2(s.z)].join('|');
const spotKey = (o) => 'o|' + f2(o.x) + '|' + f2(o.z);
const dist = (a, b) => Math.hypot(a.x - b.x, a.z - b.z);
const byKey = (a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0);

const idle = (a) => a.state === 'idle';
// working agents first (sessions, members, interns, subs), then resting members, then other resting agents
const priority = (a) => (idle(a) ? (a.kind === 'team' ? 10 : 11) : (RANK[a.kind] ?? 2));

// seats of a pavilion type that count as working desks for any busy agent
const isWork = (s) => s.kind === 'desk' && WORK_TYPES.includes(s.type);
const near = (list, at) => (at ? list.map((s) => [dist(s, at), s]).sort((p, q) => p[0] - q[0]).map((p) => p[1]) : list);
// workspace and research desks nearest first, team desks (they belong to members) last
const nearest = (list, at) => near(list.filter((s) => s.type !== 'team'), at).concat(near(list.filter((s) => s.type === 'team'), at));

// seats an agent may use in preference order. members: their own team desk, then desks nearby. interns: the
// stools of their member, then the free desks nearest to it. subs: the kind of desk their type wants, then any work desk.
function poolFor(a, layout, desks, got) {
  if (a.kind === 'main') return desks.filter((s) => s.type === 'cabin' && s.kind === 'desk' && s.account === a.account);
  const work = desks.filter(isWork);
  if (a.kind === 'team') {
    const own = teamDesk(layout, desks, a.team);
    return (own ? [own] : []).concat(nearest(work.filter((s) => s !== own), own || null));
  }
  if (a.kind === 'intern') {
    const stools = desks.filter((s) => s.kind === 'stool' && s.member === a.team);
    const m = got.get('team:' + a.team);
    const at = m || stools[0] || anchorOf(a, wantOf(a), layout);
    return stools.concat(nearest(work, at));
  }
  const w = wantOf(a);
  const own = desks.filter((s) => s.type === w.type && s.kind === w.kind);
  return own.concat(nearest(work.filter((s) => !own.includes(s)), anchorOf(a, w, layout)));
}

// which seat kind and pavilion type a subagent type wants
function wantOf(agent) {
  if (agent.kind === 'main') return { type: 'cabin', kind: 'desk' };
  // a team member without a team desk (older layout) falls back to the workspace
  if (agent.kind === 'team') return { type: 'workspace', kind: 'desk' };
  const t = String(agent.type || '').toLowerCase();
  if (t === 'explore') return { type: 'research', kind: 'desk' };
  if (t === 'plan') return { type: 'treehouse', kind: 'meeting' };
  if (/review|test|hunter|analy/.test(t)) return { type: 'treehouse', kind: 'review' };
  return { type: 'workspace', kind: 'desk' };
}

// the desk of a team member: the seat tagged with its id, else the desk at its index in members
function teamDesk(layout, seats, id) {
  for (const p of layout.pavilions || []) {
    if (p.type !== 'team' || !Array.isArray(p.members)) continue;
    const i = p.members.indexOf(id);
    if (i < 0) continue;
    const desks = seats.filter((s) => s.pavilionId === p.id && s.kind === 'desk');
    return desks.find((s) => s.member === id) || desks[i] || null;
  }
  return null;
}

const resolve = (layout, garden) => {
  const g = garden || (source ? source(layout) : null) || {};
  return { seats: g.seats || [], overflow: g.overflow || layout.overflow || [] };
};

const seatEntry = (s) => ({ kind: 'seat', level: s.y || 0, x: s.x, z: s.z, rot: s.rot || 0, room: s.pavilionId, seat: seatKey(s), world: true });
const standEntry = (s, room, seat) => ({ kind: 'stand', level: s.y || 0, x: s.x, z: s.z, rot: s.rot || 0, room, seat, world: true });

// the point an agent wants to be near: its cabin, or the middle of the pavilions of its room type
function anchorOf(agent, want, layout) {
  let list = (layout.pavilions || []).filter((p) => p.type === want.type);
  if (agent.kind === 'main') list = list.filter((p) => p.account === agent.account);
  if (!list.length) list = (layout.pavilions || []).filter((p) => p.type === 'lobby');
  if (!list.length) return null;
  return { x: list.reduce((n, p) => n + p.x, 0) / list.length, z: list.reduce((n, p) => n + p.z, 0) / list.length };
}

const LOUNGE = ['cafe', 'play', 'lobby'];
const WORK_TYPES = ['workspace', 'research', 'team'];

// planSeats for v2. same contract as the v1 planner, entries have world coordinates.
// working agents get desks first (stools, then free desks nearby, for interns), resting ones take what is
// left and otherwise wait in the cafe, play area or lobby. state "arriving" sends an agent to the lobby first.
export function planGarden(layout, roster, prev, garden) {
  const { seats, overflow } = resolve(layout, garden);
  const desks = seats.filter((s) => s.kind !== 'lobby' && s.kind !== 'cafe' && s.kind !== 'play');
  const lobby = seats.filter((s) => s.kind === 'lobby');
  const spots = new Map(overflow.map((o) => [spotKey(o), o]));
  const first = new Map();
  for (const a of roster) if (!first.has(a.key)) first.set(a.key, a);
  const everyone = [...first.values()].sort((a, b) => (priority(a) - priority(b)) || byKey(a, b));
  const plan = new Map();
  const claimed = new Set();
  const take = (a, list, keyOf, make) => {
    const s = list.find((x) => !claimed.has(keyOf(x)));
    if (!s) return false;
    claimed.add(keyOf(s));
    plan.set(a.key, make(s));
    return true;
  };
  const toLobby = (a) => take(a, lobby, seatKey, (s) => standEntry(s, s.pavilionId, seatKey(s)));
  const toSpot = (a) => {
    const old = prev?.get(a.key);
    const kept = old?.kind === 'stand' ? spots.get(old.seat) : null;
    if (kept && !claimed.has(old.seat)) { claimed.add(old.seat); plan.set(a.key, standEntry(kept, null, old.seat)); return true; }
    const at = anchorOf(a, wantOf(a), layout);
    const free = overflow.filter((o) => !claimed.has(spotKey(o)));
    if (at) free.sort((p, q) => dist(p, at) - dist(q, at));
    return take(a, free, spotKey, (o) => standEntry(o, null, spotKey(o)));
  };

  // fresh arrivals walk in through the lobby before they get a desk
  const fresh = (a) => a.state === 'arriving' && a.kind !== 'team' && prev?.get(a.key)?.kind !== 'seat';
  const arrived = everyone.filter((a) => fresh(a) && toLobby(a));
  const agents = everyone.filter((a) => !arrived.includes(a));

  // the allocator holds desks and stools in one pool, claimed ones from the lobby step stay out of it
  const pool = (a, got) => poolFor(a, layout, desks, got).filter((s) => !claimed.has(seatKey(s)));
  // a member on a desk that is not its own goes home as soon as its team desk is free
  const carryOk = (a, s, isFree) => {
    const own = a.kind === 'team' ? teamDesk(layout, desks, a.team) : null;
    return !own || s === own || !isFree(own);
  };
  const { got } = allocDesks({ agents, prev, seats: desks, idOf: seatKey, pool, idle, evictable: (a) => a.kind !== 'main', carryOk });
  for (const [key, s] of got) plan.set(key, seatEntry(s));

  // nobody found a desk: interns past the stools are hidden behind a badge, the rest wait
  const stoolMembers = new Set(desks.filter((s) => s.kind === 'stool').map((s) => s.member));
  const loose = [];
  const resting = [];
  for (const a of agents) {
    if (plan.has(a.key)) continue;
    if (a.kind === 'intern' && stoolMembers.has(a.team)) plan.set(a.key, { kind: 'hidden', badgeFor: a.team, member: 'team:' + a.team });
    else if (idle(a)) resting.push(a);
    else if (!toSpot(a) && !toLobby(a)) loose.push(a);
  }
  const lounge = seats.filter((s) => LOUNGE.includes(s.kind));
  const loungeBy = new Map(lounge.map((s) => [seatKey(s), s]));
  for (const a of resting) {
    const old = prev?.get(a.key);
    const kept = old?.kind === 'stand' ? loungeBy.get(old.seat) : null;
    if (kept && !claimed.has(old.seat)) { claimed.add(old.seat); plan.set(a.key, standEntry(kept, kept.pavilionId, old.seat)); continue; }
    const ordered = LOUNGE.flatMap((k) => lounge.filter((s) => s.kind === k));
    if (!take(a, ordered, seatKey, (s) => standEntry(s, s.pavilionId, seatKey(s))) && !toSpot(a)) loose.push(a);
  }
  const home = (layout.pavilions || []).find((p) => p.type === 'lobby') || (layout.pavilions || [])[0] || { x: 0, z: 0 };
  const pad = lobby.length ? lobby.map((s) => ({ x: s.x, z: s.z, rot: s.rot, y: s.y })) : [{ x: home.x, z: home.z + 1.5, rot: 0, y: 0 }];
  loose.forEach((a, i) => plan.set(a.key, standEntry(pad[i % pad.length], home.id || null, null)));
  return plan;
}

// break spots for v2: cafe and play seats, the lobby when the campus has neither
export function gardenBreakSpots(layout, garden) {
  const { seats } = resolve(layout, garden);
  const make = (kind) => (s) => ({ kind, level: s.y || 0, x: s.x, z: s.z, rot: s.rot || 0, room: s.pavilionId, seat: seatKey(s), world: true });
  const cafe = seats.filter((s) => s.kind === 'cafe').map(make('cafe'));
  const play = seats.filter((s) => s.kind === 'play').map(make('play'));
  let all = cafe.concat(play);
  if (!all.length) all = seats.filter((s) => s.kind === 'lobby').map(make('lobby'));
  return { cafe, play, all };
}
