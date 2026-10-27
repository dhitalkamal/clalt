// seat planning for team members and interns: team pavilion (v2) and the hall (v1)
import test from 'node:test';
import assert from 'node:assert/strict';
import { planSeats, badgeCounts } from '../routing.js';

const seat = (pavilionId, type, kind, x, z, extra = {}) => ({ pavilionId, type, account: '', y: 0, x, z, rot: 0, kind, ...extra });
const MEMBERS = ['general-purpose', 'explore', 'plan', 'code-reviewer'];

// a campus with a team pavilion: one desk per member in order and 3 stools per desk, a lobby and a cabin
function campus({ members = MEMBERS, desks = members.length, stools = 3, workspace = 2 } = {}) {
  const seats = [seat('cab', 'cabin', 'desk', 0, 0, { account: 'a' }), seat('cab', 'cabin', 'desk', 2, 0, { account: 'a' })];
  for (let i = 0; i < desks; i++) {
    seats.push(seat('tm', 'team', 'desk', 50 + i * 2, 0, { id: 'tm:desk-0-' + i }));
    for (let k = 0; k < stools; k++) seats.push(seat('tm', 'team', 'stool', 50 + i * 2 + 0.6, k * 0.5, { id: `tm:stool-${i}-${k}`, member: members[i] }));
  }
  for (let i = 0; i < workspace; i++) seats.push(seat('ws', 'workspace', 'desk', 20 + i * 2, 10));
  seats.push(seat('lob', 'lobby', 'lobby', -5, 0), seat('lob', 'lobby', 'lobby', -5, 1));
  return {
    v: 2, name: 't',
    pavilions: [{ id: 'lob', type: 'lobby', x: -5, z: 0 }, { id: 'cab', type: 'cabin', x: 0, z: 0, account: 'a' }, { id: 'tm', type: 'team', x: 50, z: 0, members },
      { id: 'ws', type: 'workspace', x: 20, z: 10 }],
    overflow: [{ x: 30, z: 23, rot: 0, near: 'workspace' }],
    seats,
  };
}
const plan = (l, roster, prev) => planSeats(l, roster, null, prev, { seats: l.seats, overflow: l.overflow });
const member = (id, state = 'idle') => ({ key: 'team:' + id, kind: 'team', team: id, type: id, state });
const intern = (key, team, owner = 's1') => ({ key, kind: 'intern', team, owner, account: 'a', type: team, state: 'busy' });
const main = (key) => ({ key, kind: 'main', account: 'a', type: 'main', state: 'busy' });
const team = () => MEMBERS.map((m) => member(m));

test('v2 members take the desk of their place in the members list', () => {
  const l = campus();
  const p = plan(l, team().reverse());
  MEMBERS.forEach((m, i) => {
    const e = p.get('team:' + m);
    assert.equal(e.kind, 'seat');
    assert.equal(e.room, 'tm');
    assert.equal(e.seat, 'tm:desk-0-' + i);
  });
});

test('v2 members and sessions do not collide and sessions keep their cabin desk', () => {
  const p = plan(campus(), [...team(), main('s1')]);
  assert.equal(p.get('s1').room, 'cab');
  assert.equal(new Set([...p.values()].map((e) => e.seat)).size, p.size);
});

test('v2 a member the layout does not list falls back to a workspace desk', () => {
  const l = campus({ members: MEMBERS.slice(0, 3), desks: 3 });
  const p = plan(l, team());
  assert.equal(p.get('team:code-reviewer').room, 'ws');
  assert.equal(p.get('team:plan').room, 'tm');
});

test('v2 interns take the stools of their agent desk, three at most', () => {
  const l = campus();
  const roster = [...team(), intern('s1/a', 'explore'), intern('s1/b', 'explore'), intern('s1/c', 'explore')];
  const p = plan(l, roster);
  const stools = ['s1/a', 's1/b', 's1/c'].map((k) => p.get(k));
  for (const e of stools) { assert.equal(e.kind, 'seat'); assert.match(e.seat, /^tm:stool-1-/); }
  assert.equal(new Set(stools.map((e) => e.seat)).size, 3);
});

test('v2 an intern past the stools and every work desk is hidden and counts toward the badge', () => {
  const l = campus({ workspace: 0 });
  const roster = [...MEMBERS.map((m) => member(m, 'busy')), ...['a', 'b', 'c', 'd', 'e'].map((k) => intern('s1/' + k, 'plan'))];
  const p = plan(l, roster);
  const hidden = [...p.entries()].filter(([, e]) => e.kind === 'hidden');
  assert.equal(hidden.length, 2);
  assert.deepEqual(hidden[0][1], { kind: 'hidden', badgeFor: 'plan', member: 'team:plan' });
  assert.deepEqual([...badgeCounts(p)], [['plan', 2]]);
});

test('v2 interns keep their stool between polls and a new intern does not push anyone', () => {
  const l = campus();
  const r1 = [...team(), intern('s1/a', 'explore'), intern('s1/b', 'explore')];
  const p1 = plan(l, r1);
  const p2 = plan(l, [...r1, intern('s1/0', 'explore')], p1);
  assert.equal(p2.get('s1/a').seat, p1.get('s1/a').seat);
  assert.equal(p2.get('s1/b').seat, p1.get('s1/b').seat);
});

test('v2 interns of a member without a team desk take a free work desk', () => {
  const l = campus({ members: MEMBERS.slice(0, 3), desks: 3 });
  const p = plan(l, [...team(), intern('s1/a', 'code-reviewer')]);
  assert.equal(p.get('s1/a').kind, 'seat');
  assert.notEqual(p.get('s1/a').room, 'tm');
});

test('v2 interns of a member without a team desk wait in the lobby when no desk is free', () => {
  const l = campus({ members: MEMBERS.slice(0, 3), desks: 3, workspace: 0 });
  const p = plan(l, [...MEMBERS.map((m) => member(m, 'busy')), intern('s1/a', 'code-reviewer')]);
  assert.equal(p.get('s1/a').kind, 'stand');
});

test('v2 interns of an unknown team do not crash and wait somewhere', () => {
  const p = plan(campus(), [...team(), intern('s1/a', 'ghost')]);
  assert.ok(p.get('s1/a'));
  assert.notEqual(p.get('s1/a').kind, 'hidden');
});

test('v2 plan without any team behaves as before', () => {
  const p = plan(campus(), [main('s1')]);
  assert.equal(p.get('s1').room, 'cab');
});

// the hall (v1): team members use the workspace desks and then the research desks
const CATALOG = [{ id: 'desk', footprint: { w: 2, d: 1 }, tags: ['seat'] }];
let fid = 0;
const room = (id, type, x, z) => ({ id, type, level: 0, x, z, w: 10, d: 8, label: id });
const desksIn = (r, n) => Array.from({ length: n }, (_, i) => ({ id: 'f' + (fid++), item: 'desk', level: 0, x: r.x + 1 + (i % 4) * 2, z: r.z + 1 + Math.floor(i / 4) * 3, rot: 0, room: r.id }));
function hall(ws = 3, res = 2) {
  const rooms = [room('lobby', 'lobby', 0, 0), room('cab', 'cabins', 12, 0), room('res', 'research', 24, 0), room('ws', 'workspace', 0, 10)];
  const furniture = [...desksIn(rooms[1], 2), ...desksIn(rooms[2], res), ...desksIn(rooms[3], ws)];
  return { v: 1, name: 't', grid: 0.5, size: { w: 60, d: 40 }, door: { side: 'left', at: 2 }, deck: null, stairs: null, rooms, furniture };
}
const hallPlan = (l, roster, prev) => planSeats(l, roster, CATALOG, prev);

test('hall members take workspace desks first, then research desks, extras stand in the lobby', () => {
  const l = hall(3, 2);
  const ids = ['m1', 'm2', 'm3', 'm4', 'm5', 'm6'];
  const p = hallPlan(l, ids.map((i) => member(i)));
  assert.deepEqual(ids.map((i) => p.get('team:' + i).room), ['ws', 'ws', 'ws', 'res', 'res', 'lobby']);
  assert.equal(p.get('team:m6').kind, 'stand');
});

test('hall busy members and subagents get desks before idle members and sessions keep theirs', () => {
  const l = hall(2, 0);
  const roster = [main('s1'), { key: 's1/x', kind: 'sub', account: 'a', type: 'mystery', state: 'busy' }, member('m1', 'busy'), member('m2')];
  const p = hallPlan(l, roster);
  assert.equal(p.get('team:m1').room, 'ws');
  assert.equal(p.get('s1/x').room, 'ws');
  assert.equal(p.get('s1').room, 'cab');
  assert.equal(p.get('team:m2').kind, 'stand');
});

test('hall interns stand on free floor beside their member desk', () => {
  const l = hall(2, 0);
  const roster = [member('m1', 'busy'), member('m2', 'busy'), intern('s1/a', 'm2'), intern('s1/b', 'm2')];
  const p = hallPlan(l, roster);
  const m = p.get('team:m2');
  for (const k of ['s1/a', 's1/b']) {
    const e = p.get(k);
    assert.equal(e.kind, 'stand');
    assert.equal(e.room, m.room);
    assert.ok(Math.hypot(e.x - m.x, e.z - m.z) <= 2.3, k);
  }
  const a = p.get('s1/a');
  const b = p.get('s1/b');
  assert.ok(a.x !== b.x || a.z !== b.z);
  const blocked = new Set(l.furniture.flatMap((f) => [f.x, f.x + 1].map((x) => x + ',' + f.z)));
  for (const e of [a, b]) assert.ok(!blocked.has(Math.floor(e.x) + ',' + Math.floor(e.z)));
});

test('hall interns of a member in the lobby stand in the lobby', () => {
  const l = hall(0, 0);
  const p = hallPlan(l, [member('m1'), intern('s1/a', 'm1')]);
  assert.equal(p.get('s1/a').room, 'lobby');
});
