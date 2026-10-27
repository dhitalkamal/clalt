// working agents sit at desks first: hall (v1) and garden (v2) seat priority, lounge for the rest, stable seats
import test from 'node:test';
import assert from 'node:assert/strict';
import { planSeats } from '../routing.js';
import { hallLayout, hallCatalog } from './hall-fixture.js';

const member = (id, state = 'idle') => ({ key: 'team:' + id, kind: 'team', team: id, type: id, state });
const intern = (key, team) => ({ key, kind: 'intern', team, owner: 's1', account: 'a', type: team, state: 'busy' });
const sub = (key, type = 'general-purpose') => ({ key, kind: 'sub', account: 'a', type, state: 'busy' });
const ids = (n) => Array.from({ length: n }, (_, i) => 'm' + i);
const WORK = ['workspace', 'research'];

// synthetic hall: cabins, lobby, cafe, workspace and research with desks in rows of four
const CATALOG = [{ id: 'desk', footprint: { w: 2, d: 1 }, tags: ['seat'] }];
let fid = 0;
const room = (id, type, x, z) => ({ id, type, level: 0, x, z, w: 10, d: 8, label: id });
const desksIn = (r, n) => Array.from({ length: n }, (_, i) => ({ id: 'f' + (fid++), item: 'desk', level: 0, x: r.x + 1 + (i % 4) * 2, z: r.z + 1 + Math.floor(i / 4) * 3, rot: 0, room: r.id }));
function hall(ws = 4, res = 0) {
  const rooms = [room('lobby', 'lobby', 0, 0), room('cab', 'cabins', 12, 0), room('res', 'research', 24, 0), room('ws', 'workspace', 0, 10), room('cafe', 'cafe', 12, 10)];
  const furniture = [...desksIn(rooms[1], 2), ...desksIn(rooms[2], res), ...desksIn(rooms[3], ws)];
  return { v: 1, name: 't', grid: 0.5, size: { w: 60, d: 40 }, door: { side: 'left', at: 2 }, deck: null, stairs: null, rooms, furniture };
}
const plan = (l, roster, prev, cat = CATALOG) => planSeats(l, roster, cat, prev);
const seated = (e) => e.kind === 'seat' && WORK.includes(e.room === 'ws' ? 'workspace' : e.room === 'res' ? 'research' : e.room);
const dist = (a, b) => Math.hypot(a.x - b.x, a.z - b.z);

test('hall: 11 members (2 busy) and 2 interns on the real layout, workers at desks, idle extras in the lounge', () => {
  const l = hallLayout();
  const members = ids(11);
  const roster = members.map((m, i) => member(m, i === 8 || i === 10 ? 'busy' : 'idle'));
  roster.push(intern('s1/a', 'm8'), intern('s1/b', 'm10'));
  const p = plan(l, roster, null, hallCatalog());
  const rooms = new Map(l.rooms.map((r) => [r.id, r.type]));
  for (const k of ['team:m8', 'team:m10', 's1/a', 's1/b']) {
    assert.equal(p.get(k).kind, 'seat', k);
    assert.ok(WORK.includes(rooms.get(p.get(k).room)), k);
  }
  const standing = [...p.entries()].filter(([, e]) => e.kind === 'stand');
  for (const [, e] of standing) assert.ok(['cafe', 'play', 'lobby'].includes(rooms.get(e.room)));
  // the idle ones that fit sit, the rest stand in the lounge area
  assert.equal(standing.length + [...p.values()].filter((e) => e.kind === 'seat').length, 13);
});

test('hall: idle members never hold a desk a busy member needs', () => {
  const l = hall(4, 0);
  const idleOnly = ids(6).map((m) => member(m));
  const p1 = plan(l, idleOnly);
  assert.equal([...p1.values()].filter((e) => e.kind === 'seat').length, 4);
  // two members become busy: they take desks, two idle ones are bumped to the cafe
  const roster = idleOnly.map((a) => (a.team === 'm4' || a.team === 'm5' ? { ...a, state: 'busy' } : a));
  const p2 = plan(l, roster, p1);
  assert.equal(p2.get('team:m4').kind, 'seat');
  assert.equal(p2.get('team:m5').kind, 'seat');
  assert.equal([...p2.values()].filter((e) => e.kind === 'seat').length, 4);
  for (const k of ['team:m4', 'team:m5']) assert.equal(p2.get(k).room, 'ws');
});

test('hall: a busy agent seated before is never moved for an idle one', () => {
  const l = hall(3, 0);
  const r1 = [member('m0', 'busy'), member('m1', 'busy'), member('m2', 'busy'), member('m3')];
  const p1 = plan(l, r1);
  assert.equal(p1.get('team:m3').kind, 'stand');
  // m3 stays idle, a busy sub arrives: it needs a desk but all three belong to busy members
  const p2 = plan(l, [...r1, sub('s1/x')], p1);
  for (const k of ['team:m0', 'team:m1', 'team:m2']) assert.deepEqual(p2.get(k), p1.get(k));
  assert.equal(p2.get('s1/x').kind, 'stand');
  assert.equal(p2.get('team:m3').kind, 'stand');
});

test('hall: same input twice and repeated planning give the same plan', () => {
  const l = hall(4, 0);
  const roster = [...ids(7).map((m, i) => member(m, i % 3 === 0 ? 'busy' : 'idle')), intern('s1/a', 'm0'), sub('s1/x')];
  const p1 = plan(l, roster);
  let prev = p1;
  for (let i = 0; i < 4; i++) prev = plan(l, roster, prev);
  assert.deepEqual([...prev.entries()], [...p1.entries()]);
  assert.deepEqual([...plan(l, [...roster].reverse(), p1).entries()].sort(), [...p1.entries()].sort());
});

test('hall: a member going idle keeps its desk until a busy agent needs it', () => {
  const l = hall(3, 0);
  const r1 = [member('m0', 'busy'), member('m1', 'busy'), member('m2')];
  const p1 = plan(l, r1);
  const r2 = [member('m0'), member('m1', 'busy'), member('m2')];
  const p2 = plan(l, r2, p1);
  assert.deepEqual(p2.get('team:m0'), p1.get('team:m0'));
  // a busy sub arrives and every desk is taken or held idle: it bumps an idle holder
  const r3 = [...r2, sub('s1/x'), sub('s1/y')];
  const p3 = plan(l, r3, p2);
  assert.equal(p3.get('s1/x').kind, 'seat');
  assert.equal(p3.get('s1/y').kind, 'seat');
  assert.deepEqual(p3.get('team:m1'), p2.get('team:m1'));
  assert.equal([p3.get('team:m0'), p3.get('team:m2')].filter((e) => e.kind === 'stand').length, 2);
});

test('hall: a waiting idle member takes the desk a busy agent frees', () => {
  const l = hall(2, 0);
  const r1 = [sub('s1/x'), sub('s1/y'), member('m0')];
  const p1 = plan(l, r1);
  assert.equal(p1.get('team:m0').kind, 'stand');
  const p2 = plan(l, [sub('s1/y'), member('m0')], p1);
  assert.equal(p2.get('team:m0').kind, 'seat');
  assert.deepEqual(p2.get('s1/y'), p1.get('s1/y'));
});

test('hall: interns take the free desk nearest to their member, then any free work desk', () => {
  const l = hall(8, 0);
  const roster = [member('m0', 'busy'), intern('s1/a', 'm0'), intern('s1/b', 'm0')];
  const p = plan(l, roster);
  const m = p.get('team:m0');
  for (const k of ['s1/a', 's1/b']) assert.equal(p.get(k).kind, 'seat');
  assert.ok(dist(m, p.get('s1/a')) <= dist(m, p.get('s1/b')));
  const others = l.furniture.filter((f) => f.room === 'ws').map((f) => ({ x: f.x + 1, z: f.z + 0.5 }))
    .filter((c) => !['team:m0', 's1/a', 's1/b'].some((k) => p.get(k).x === c.x && p.get(k).z === c.z));
  for (const k of ['s1/a', 's1/b']) for (const c of others) assert.ok(dist(m, p.get(k)) <= dist(m, c) + 1e-9, k);
});

test('hall: interns use a desk in another room, then stand beside the member as a last resort', () => {
  const l = hall(1, 2);
  const roster = [member('m0', 'busy'), intern('s1/a', 'm0'), intern('s1/b', 'm0'), intern('s1/c', 'm0')];
  const p = plan(l, roster);
  assert.equal(p.get('team:m0').room, 'ws');
  assert.deepEqual(['s1/a', 's1/b'].map((k) => p.get(k).room), ['res', 'res']);
  assert.equal(p.get('s1/c').kind, 'stand');
  assert.equal(p.get('s1/c').room, 'ws');
});

test('hall: idle members rest in the cafe when the room exists', () => {
  const l = hall(1, 0);
  const p = plan(l, [member('m0'), member('m1'), member('m2')]);
  assert.equal(p.get('team:m0').kind, 'seat');
  for (const k of ['team:m1', 'team:m2']) assert.equal(p.get(k).room, 'cafe');
});

// garden: a team pavilion with 2 desks, 3 stools each, plus a workspace and a cafe
const seat = (pavilionId, type, kind, x, z, extra = {}) => ({ pavilionId, type, account: '', y: 0, x, z, rot: 0, kind, ...extra });
const MEM = ['alpha', 'beta'];
function campus({ workspace = 1 } = {}) {
  const seats = [seat('cab', 'cabin', 'desk', 0, 0, { account: 'a' })];
  MEM.forEach((m, i) => {
    seats.push(seat('tm', 'team', 'desk', 50 + i * 4, 0, { id: 'tm:desk-' + i }));
    for (let k = 0; k < 3; k++) seats.push(seat('tm', 'team', 'stool', 50 + i * 4 + 1, k * 0.5, { id: `tm:stool-${i}-${k}`, member: m }));
  });
  for (let i = 0; i < workspace; i++) seats.push(seat('ws', 'workspace', 'desk', 20 + i * 2, 10));
  seats.push(seat('cafe', 'cafe', 'cafe', 30, 20), seat('lob', 'lobby', 'lobby', -5, 0));
  return { v: 2, name: 't', pavilions: [{ id: 'lob', type: 'lobby', x: -5, z: 0 }, { id: 'tm', type: 'team', x: 50, z: 0, members: MEM },
    { id: 'ws', type: 'workspace', x: 20, z: 10 }, { id: 'cafe', type: 'cafe', x: 30, z: 20 }, { id: 'cab', type: 'cabin', x: 0, z: 0, account: 'a' }],
  overflow: [{ x: 30, z: 23, rot: 0, near: 'workspace' }], seats };
}
const gplan = (l, roster, prev) => planSeats(l, roster, null, prev, { seats: l.seats, overflow: l.overflow });

test('garden: full stools send interns to free desks nearby, never to a stand', () => {
  const l = campus({ workspace: 3 });
  const roster = [member('alpha', 'busy'), member('beta'), ...['a', 'b', 'c', 'd', 'e'].map((k) => intern('s1/' + k, 'alpha'))];
  const p = gplan(l, roster);
  const kinds = ['a', 'b', 'c', 'd', 'e'].map((k) => p.get('s1/' + k));
  assert.deepEqual(kinds.map((e) => e.kind), Array(5).fill('seat'));
  assert.equal(kinds.filter((e) => /stool/.test(e.seat)).length, 3);
  // the idle member keeps its own desk, the extra interns used workspace desks
  assert.equal(p.get('team:beta').seat, 'tm:desk-1');
  assert.deepEqual(kinds.filter((e) => !/stool/.test(e.seat)).map((e) => e.room), ['ws', 'ws']);
});

test('garden: a busy agent bumps an idle member to the cafe only when no other desk is free', () => {
  const l = campus({ workspace: 1 });
  const idle = [member('alpha'), member('beta')];
  const p1 = gplan(l, [...idle, sub('s1/x')]);
  assert.equal(p1.get('s1/x').room, 'ws');
  assert.equal(p1.get('team:alpha').seat, 'tm:desk-0');
  const r2 = [...idle, sub('s1/x'), sub('s1/y')];
  const p2 = gplan(l, r2, p1);
  assert.equal(p2.get('s1/y').kind, 'seat');
  assert.deepEqual(p2.get('s1/x'), p1.get('s1/x'));
  const bumped = ['team:alpha', 'team:beta'].filter((k) => p2.get(k).kind === 'stand');
  assert.equal(bumped.length, 1);
  assert.equal(p2.get(bumped[0]).room, 'cafe');
});

test('garden: busy members take their own desk and repeated planning is stable', () => {
  const l = campus({ workspace: 1 });
  const roster = [member('alpha', 'busy'), member('beta', 'busy'), sub('s1/x'), sub('s1/y'), intern('s1/a', 'beta')];
  const p1 = gplan(l, roster);
  assert.equal(p1.get('team:alpha').seat, 'tm:desk-0');
  assert.equal(p1.get('team:beta').seat, 'tm:desk-1');
  let prev = p1;
  for (let i = 0; i < 4; i++) prev = gplan(l, roster, prev);
  assert.deepEqual([...prev.entries()], [...p1.entries()]);
});

test('hall: floor decor does not block the cafe, idle members stand in it on the real layout', () => {
  const l = hallLayout();
  const roster = ids(20).map((m) => member(m));
  const p = plan(l, roster, null, hallCatalog());
  const lounge = [...p.values()].filter((e) => e.kind === 'stand');
  assert.ok(lounge.length > 0);
  assert.ok(lounge.some((e) => e.room === 'cafe'));
});
