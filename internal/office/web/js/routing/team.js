// team parts of seat planning: where interns stand in the hall, and the +N badge counts.
// kinds: roster entries kind 'team' (a member) and 'intern' (a later run of a member's type).
import { blockedCells } from './spatial.js';

// plan order of roster kinds: sessions, then team members, then generic subagents, interns last
export const RANK = { main: 0, team: 1, sub: 2, intern: 3 };

// hidden interns per team id in a plan: Map(teamId -> count), for the +N badge at the desk
export function badgeCounts(plan) {
  const out = new Map();
  for (const e of plan.values()) if (e.kind === 'hidden') out.set(e.badgeFor, (out.get(e.badgeFor) || 0) + 1);
  return out;
}

const SIDES = [[0, 1], [0, -1], [1, 0], [-1, 0], [1, 1], [-1, 1], [1, -1], [-1, -1], [0, 2], [0, -2]];
const cellKey = (level, x, z) => level + ',' + Math.floor(x) + ',' + Math.floor(z);

// floor cells next to a member's desk that are free of furniture and inside the member's room
function besideCells(layout, blocked, seat) {
  const room = (layout.rooms || []).find((r) => r.id === seat.room);
  if (!room) return [];
  const cx = Math.floor(seat.x) + 0.5;
  const cz = Math.floor(seat.z) + 0.5;
  const out = [];
  for (const [dx, dz] of SIDES) {
    const x = cx + dx;
    const z = cz + dz;
    const inside = x > room.x && x < room.x + room.w && z > room.z && z < room.z + room.d;
    if (inside && !blocked.has(cellKey(seat.level, x, z))) out.push({ level: seat.level, x, z, rot: 0, room: room.id });
  }
  return out;
}

// hall: interns stand beside the desk of their member. plan holds the members already.
// returns the interns that found no cell (they join the lobby crowd).
export function placeHallInterns(layout, by, interns, plan, prev) {
  const blocked = blockedCells(layout, by);
  const taken = new Set();
  const left = [];
  const cands = new Map();
  const cellsOf = (a) => {
    const key = 'team:' + a.team;
    if (!cands.has(key)) {
      const m = plan.get(key);
      cands.set(key, m && m.kind === 'seat' ? besideCells(layout, blocked, m) : []);
    }
    return cands.get(key);
  };
  const entry = (a, c) => ({ kind: 'stand', level: c.level, x: c.x, z: c.z, rot: 0, room: c.room, beside: 'team:' + a.team });
  const rest = [];
  for (const a of interns) {
    const old = prev?.get(a.key);
    const c = old?.kind === 'stand' ? cellsOf(a).find((k) => k.x === old.x && k.z === old.z && k.level === old.level) : null;
    if (c && !taken.has(cellKey(c.level, c.x, c.z))) { taken.add(cellKey(c.level, c.x, c.z)); plan.set(a.key, entry(a, c)); } else rest.push(a);
  }
  for (const a of rest) {
    const c = cellsOf(a).find((k) => !taken.has(cellKey(k.level, k.x, k.z)));
    if (c) { taken.add(cellKey(c.level, c.x, c.z)); plan.set(a.key, entry(a, c)); } else left.push(a);
  }
  return left;
}
