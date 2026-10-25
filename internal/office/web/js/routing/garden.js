// seat planning for layout v2 (garden campus). pure functions over the seat list the renderer builds,
// no dom, no three.js. coordinates are world units, entries carry world: true so callers do not
// treat them as grid cells.
import { RANK } from './team.js';

// where the seat list comes from. the page registers gardenSeats here at boot; tests pass a source in.
let source = null;
export function setGardenSource(fn) { source = fn; }

const f2 = (n) => (Math.round(n * 100) / 100).toFixed(2);
// the renderer's seat id names a seat; one without an id is named by pavilion, kind and place
const seatKey = (s) => s.id || [s.pavilionId, s.kind, f2(s.x), f2(s.z)].join('|');
const spotKey = (o) => 'o|' + f2(o.x) + '|' + f2(o.z);
const dist = (a, b) => Math.hypot(a.x - b.x, a.z - b.z);
const byKey = (a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0);

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

// interns sit on the stools of their member's team desk (seats of kind stool carrying the member id).
// beyond the stools they are hidden and counted in the badge. a member without a team desk or stools: fallback.
function placeInterns(interns, seats, plan, prev, claimed, fallback) {
  const stoolsOf = new Map();
  for (const s of seats) if (s.kind === 'stool') { if (!stoolsOf.has(s.member)) stoolsOf.set(s.member, []); stoolsOf.get(s.member).push(s); }
  const onDesk = (a) => {
    const m = plan.get('team:' + a.team);
    return m && m.kind === 'seat' && (stoolsOf.get(a.team) || []).length > 0 && seats.some((s) => seatKey(s) === m.seat && s.type === 'team');
  };
  const rest = [];
  for (const a of interns) {
    if (!onDesk(a)) { fallback(a); continue; }
    const old = prev?.get(a.key);
    const s = old?.kind === 'seat' ? stoolsOf.get(a.team).find((x) => seatKey(x) === old.seat) : null;
    if (s && !claimed.has(seatKey(s))) { claimed.add(seatKey(s)); plan.set(a.key, seatEntry(s)); } else rest.push(a);
  }
  for (const a of rest) {
    const s = stoolsOf.get(a.team).find((x) => !claimed.has(seatKey(x)));
    if (s) { claimed.add(seatKey(s)); plan.set(a.key, seatEntry(s)); } else plan.set(a.key, { kind: 'hidden', badgeFor: a.team, member: 'team:' + a.team });
  }
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

// planSeats for v2. same contract as the v1 planner, entries have world coordinates.
// roster state "arriving" sends an agent to the lobby first.
export function planGarden(layout, roster, prev, garden) {
  const { seats, overflow } = resolve(layout, garden);
  const desks = seats.filter((s) => s.kind !== 'lobby' && s.kind !== 'cafe' && s.kind !== 'play' && s.kind !== 'stool');
  const lobby = seats.filter((s) => s.kind === 'lobby');
  const byDesk = new Map(desks.map((s) => [seatKey(s), s]));
  const spots = new Map(overflow.map((o) => [spotKey(o), o]));
  const lobbyBy = new Map(lobby.map((s) => [seatKey(s), s]));
  const first = new Map();
  for (const a of roster) if (!first.has(a.key)) first.set(a.key, a);
  const everyone = [...first.values()].sort((a, b) => (a.kind === b.kind ? byKey(a, b) : (RANK[a.kind] ?? 2) - (RANK[b.kind] ?? 2)));
  const interns = everyone.filter((a) => a.kind === 'intern');
  const agents = everyone.filter((a) => a.kind !== 'intern');
  const fits = (a, s) => {
    const w = wantOf(a);
    return s.type === w.type && s.kind === w.kind && (a.kind !== 'main' || s.account === a.account);
  };
  const plan = new Map();
  const claimed = new Set();

  // team members sit at the desk of their place in the members list of a team pavilion
  for (const a of agents) {
    if (a.kind !== 'team') continue;
    const s = teamDesk(layout, seats, a.team);
    if (s && !claimed.has(seatKey(s))) { claimed.add(seatKey(s)); plan.set(a.key, seatEntry(s)); }
  }

  // keep previous desks that still exist and still fit
  for (const a of agents) {
    const old = prev?.get(a.key);
    const s = old?.kind === 'seat' ? byDesk.get(old.seat) : null;
    if (s && !claimed.has(old.seat) && fits(a, s)) { claimed.add(old.seat); plan.set(a.key, seatEntry(s)); }
  }

  const take = (a, list, keyOf, make) => {
    const s = list.find((x) => !claimed.has(keyOf(x)));
    if (!s) return false;
    claimed.add(keyOf(s));
    plan.set(a.key, make(s));
    return true;
  };
  const toLobby = (a) => take(a, lobby, seatKey, (s) => standEntry(s, s.pavilionId, seatKey(s)));
  const toDesk = (a) => take(a, desks.filter((s) => fits(a, s)), seatKey, seatEntry);
  const toSpot = (a) => {
    const old = prev?.get(a.key);
    const kept = old?.kind === 'stand' ? spots.get(old.seat) : null;
    if (kept && !claimed.has(old.seat)) { claimed.add(old.seat); plan.set(a.key, standEntry(kept, null, old.seat)); return true; }
    const at = anchorOf(a, wantOf(a), layout);
    const free = overflow.filter((o) => !claimed.has(spotKey(o)));
    if (at) free.sort((p, q) => dist(p, at) - dist(q, at));
    return take(a, free, spotKey, (o) => standEntry(o, null, spotKey(o)));
  };

  const loose = [];
  for (const a of agents) {
    if (plan.has(a.key)) continue;
    if (a.state === 'arriving' && toLobby(a)) continue;
    if (!toDesk(a) && !toSpot(a) && !toLobby(a)) loose.push(a);
  }
  placeInterns(interns, seats, plan, prev, claimed, (a) => (toLobby(a) || toSpot(a)) || loose.push(a));
  const home = (layout.pavilions || []).find((p) => p.type === 'lobby') || (layout.pavilions || [])[0] || { x: 0, z: 0 };
  const pool = lobby.length ? lobby.map((s) => ({ x: s.x, z: s.z, rot: s.rot, y: s.y })) : [{ x: home.x, z: home.z + 1.5, rot: 0, y: 0 }];
  loose.forEach((a, i) => plan.set(a.key, standEntry(pool[i % pool.length], home.id || null, null)));
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
