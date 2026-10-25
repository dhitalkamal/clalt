// the roster of the office: sessions, team members, generic subagents and interns. pure, no dom.
// a team member record is {id: 'team:<id>', role: 'team', team, name, desc, color, tint, state, users, leaving}.
// sub records gain team (id or null), mapped (first run of a team type, shown as the member) and
// intern (a later run, a character of its own) plus owner (handle of the session that started it).
import { hash } from './model.js';

export const teamIdOf = (type) => String(type || '').toLowerCase();

const live = (sub) => !sub.leaving;

// how many live runs a session has of a team id, across all sessions
function runsOf(sessions, id) {
  return sessions.filter((s) => !s.leaving).flatMap((s) => s.subs.filter(live).map((x) => [s, x])).filter(([, x]) => teamIdOf(x.type) === id);
}

// sets team, mapped, intern and owner on every sub. a sub that was classified once keeps its role,
// new subs of a type become the member when no live run of the type is mapped, interns otherwise
export function classifySubs(sessions, teamIds) {
  const fresh = [];
  const taken = new Set();
  for (const s of sessions) {
    for (const x of s.subs) {
      const id = teamIdOf(x.type);
      if (x.classified) {
        if (x.mapped && live(x)) taken.add(x.team);
        continue;
      }
      if (!teamIds.has(id)) { x.team = null; x.mapped = false; x.intern = false; x.classified = true; continue; }
      fresh.push([s, x, id]);
    }
  }
  fresh.sort((a, b) => (a[1].startedAt - b[1].startedAt) || (a[1].id < b[1].id ? -1 : a[1].id > b[1].id ? 1 : 0));
  for (const [s, x, id] of fresh) {
    x.classified = true;
    x.team = id;
    x.owner = s.id;
    if (x.leaving || taken.has(id)) { x.mapped = false; x.intern = true; continue; }
    x.mapped = true;
    x.intern = false;
    taken.add(id);
  }
}

const colorNum = (c) => (/^#[0-9a-f]{6}$/i.test(c || '') ? parseInt(c.slice(1), 16) : 0x7a8aa8);

// member records for a snapshot team list. records in prev are updated in place so agents keep their record.
// a member the snapshot no longer lists is kept with leaving set.
export function updateMembers(prev, team, sessions) {
  const old = new Map(prev.map((m) => [m.team, m]));
  const seen = new Set();
  const out = [];
  for (const e of team || []) {
    seen.add(e.id);
    const runs = runsOf(sessions, e.id);
    const m = old.get(e.id) || { id: 'team:' + e.id, role: 'team', team: e.id, seed: hash('team:' + e.id), account: null, subs: [] };
    const users = [];
    for (const [, x] of runs) if (x.intern) x.tint = colorNum(e.color);
    for (const [s] of runs) if (!users.some((u) => u.handle === s.handle)) users.push({ handle: s.handle, project: s.project });
    Object.assign(m, { name: e.name || e.id, desc: e.desc || '', source: e.source || '', color: e.color, tint: colorNum(e.color),
      busy: e.busy || 0, state: (e.busy || 0) > 0 || runs.length > 0 ? 'busy' : 'idle', users, leaving: false });
    out.push(m);
  }
  for (const m of prev) if (!seen.has(m.team)) { m.leaving = true; m.state = 'idle'; out.push(m); }
  return out;
}

// the entries planSeats works on. mapped runs have no entry: the member stands for them
export function rosterOf(sessions, members = []) {
  const out = [];
  for (const m of members) if (!m.leaving) out.push({ key: m.id, kind: 'team', team: m.team, type: m.team, state: m.state });
  for (const s of sessions) {
    out.push({ key: s.id, kind: 'main', account: s.account, type: 'main', state: s.state });
    for (const x of s.subs) {
      if (x.mapped) continue;
      if (x.intern) out.push({ key: x.id, kind: 'intern', team: x.team, owner: x.owner, account: s.account, type: x.type, state: 'busy' });
      else out.push({ key: x.id, kind: 'sub', account: s.account, type: x.type, state: 'busy' });
    }
  }
  return out;
}

// every record that gets a character: members, sessions and their runs, minus mapped runs and leaving subs
export function agentRecords(sessions, members = []) {
  const out = members.filter((m) => !m.leaving);
  for (const s of sessions) {
    if (s.leaving) continue;
    out.push(s, ...s.subs.filter((x) => !x.leaving && !x.mapped));
  }
  return out;
}

export function findRecord(sessions, members, key) {
  const m = members.find((x) => x.id === key);
  if (m) return m;
  for (const s of sessions) {
    if (s.id === key) return s;
    const sub = s.subs.find((x) => x.id === key);
    if (sub) return sub;
  }
  return null;
}
