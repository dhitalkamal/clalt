// routing tests. the _test folder starts with an underscore so go:embed skips it.
// run with: node --test 'internal/office/web/js/_test/*.test.js' (the bare directory form does not work).
import test from 'node:test';
import assert from 'node:assert/strict';
import { planSeats, breakSpots } from '../routing.js';

const CATALOG = [
  { id: 'desk', footprint: { w: 2, d: 1 }, tags: ['seat'] },
  { id: 'sofa', footprint: { w: 3, d: 1 }, tags: ['seat'] },
  { id: 'plant', footprint: { w: 1, d: 1 }, tags: [] },
  { id: 'ping-pong', footprint: { w: 4, d: 2 }, tags: ['play'] },
];

// a room 10 wide placed on a row, with n desks in it
let fid = 0;
function room(id, type, x, z, extra = {}) {
  return { id, type, level: 0, x, z, w: 10, d: 6, label: id, ...extra };
}
function desks(r, n, item = 'desk', rot = 0) {
  const out = [];
  for (let i = 0; i < n; i++) {
    out.push({ id: 'f' + (fid++), item, level: r.level, x: r.x + 1 + (i % 4) * 2, z: r.z + 1 + Math.floor(i / 4) * 2, rot, room: r.id });
  }
  return out;
}
function office(rooms, furniture) {
  return { v: 1, name: 't', grid: 0.5, size: { w: 60, d: 40 }, door: { side: 'left', at: 2 }, deck: null, stairs: null, rooms, furniture };
}
function full() {
  const rooms = [
    room('lobby', 'lobby', 0, 0), room('cab', 'cabins', 12, 0), room('res', 'research', 24, 0),
    room('ws', 'workspace', 0, 10), room('rev', 'review', 12, 10), room('meet', 'meeting', 24, 10),
    room('cafe', 'cafe', 36, 0), room('play', 'play', 36, 10),
  ];
  const furniture = rooms.slice(1).flatMap((r) => desks(r, 4));
  return office(rooms, furniture);
}
const sub = (key, type) => ({ key, kind: 'sub', account: 'a', type, state: 'working' });
const main = (key, account = 'a') => ({ key, kind: 'main', account, type: '', state: 'working' });
const roomOf = (plan, key) => plan.get(key).room;

test('every room type routes its agents', () => {
  const l = full();
  const roster = [main('m1'), sub('s1', 'Explore'), sub('s2', 'code-reviewer'), sub('s3', 'Plan'),
    sub('s4', 'general-purpose'), sub('s5', 'mystery'), sub('s6', 'test-runner'), sub('s7', 'bug-hunter'), sub('s8', 'code-analyzer')];
  const p = planSeats(l, roster, CATALOG);
  assert.equal(roomOf(p, 'm1'), 'cab');
  assert.equal(roomOf(p, 's1'), 'res');
  for (const k of ['s2', 's6', 's7', 's8']) assert.equal(roomOf(p, k), 'rev', k);
  assert.equal(roomOf(p, 's3'), 'meet');
  assert.equal(roomOf(p, 's4'), 'ws');
  assert.equal(roomOf(p, 's5'), 'ws');
  for (const v of p.values()) assert.equal(v.kind, 'seat');
});

test('seat point is the footprint center and faces by rot', () => {
  const r = room('lobby', 'lobby', 0, 0);
  const w = room('ws', 'workspace', 12, 0);
  const l = office([r, w], [
    { id: 'a', item: 'desk', level: 0, x: 13, z: 2, rot: 0, room: 'ws' },
    { id: 'b', item: 'desk', level: 0, x: 13, z: 4, rot: 90, room: 'ws' },
  ]);
  const p = planSeats(l, [sub('s1', 'x'), sub('s2', 'x')], CATALOG);
  assert.deepEqual([p.get('s1').x, p.get('s1').z, p.get('s1').rot], [14, 2.5, 0]);
  // rot 90 swaps the footprint to 1 wide and 2 deep
  assert.deepEqual([p.get('s2').x, p.get('s2').z, p.get('s2').rot], [13.5, 5, 90]);
});

test('seats found by position when furniture has no room, tags from furniture item', () => {
  const l = office([room('lobby', 'lobby', 0, 0), room('ws', 'workspace', 12, 0)],
    [{ id: 'a', item: 'x', tags: ['seat'], level: 0, x: 13, z: 2, rot: 0 }]);
  const p = planSeats(l, [sub('s1', 'x')]);
  assert.equal(p.get('s1').kind, 'seat');
  assert.equal(p.get('s1').room, 'ws');
});

test('non seat furniture is never a seat', () => {
  const l = office([room('lobby', 'lobby', 0, 0), room('ws', 'workspace', 12, 0)],
    [{ id: 'a', item: 'plant', level: 0, x: 13, z: 2, rot: 0, room: 'ws' }]);
  assert.equal(planSeats(l, [sub('s1', 'x')], CATALOG).get('s1').kind, 'stand');
});

test('cabins respect the account', () => {
  const rooms = [room('lobby', 'lobby', 0, 0), room('ca', 'cabins', 12, 0, { account: 'a' }),
    room('cb', 'cabins', 24, 0, { account: 'b' }), room('co', 'cabins', 36, 0)];
  const l = office(rooms, rooms.slice(1).flatMap((r) => desks(r, 1)));
  const p = planSeats(l, [main('m1', 'b'), main('m2', 'a'), main('m3', 'c'), main('m4', 'b')], CATALOG);
  assert.equal(roomOf(p, 'm1'), 'cb');
  assert.equal(roomOf(p, 'm2'), 'ca');
  assert.equal(roomOf(p, 'm3'), 'co');
  // second b main: cb is full, an unrestricted cabin is taken by m3, so nothing is left for it
  assert.equal(p.get('m4').kind, 'stand');
  // a cabin restricted to another account is never used
  assert.notEqual(roomOf(p, 'm3'), 'ca');
  assert.notEqual(roomOf(p, 'm3'), 'cb');
});

test('a main agent without a matching cabin overflows to workspace, then lobby', () => {
  const rooms = [room('lobby', 'lobby', 0, 0), room('ca', 'cabins', 12, 0, { account: 'a' }), room('ws', 'workspace', 24, 0)];
  const l = office(rooms, [...desks(rooms[1], 1), ...desks(rooms[2], 1)]);
  const p = planSeats(l, [main('m1', 'z'), main('m2', 'z')], CATALOG);
  assert.equal(roomOf(p, 'm1'), 'ws');
  assert.equal(p.get('m2').kind, 'stand');
  assert.equal(p.get('m2').room, 'lobby');
});

test('full research room overflows to workspace then lobby', () => {
  const rooms = [room('lobby', 'lobby', 0, 0), room('res', 'research', 12, 0), room('ws', 'workspace', 24, 0)];
  const l = office(rooms, [...desks(rooms[1], 2), ...desks(rooms[2], 2)]);
  const roster = ['e1', 'e2', 'e3', 'e4', 'e5'].map((k) => sub(k, 'Explore'));
  const p = planSeats(l, roster, CATALOG);
  const count = (room) => roster.filter((a) => p.get(a.key).room === room && p.get(a.key).kind === 'seat').length;
  assert.equal(count('res'), 2);
  assert.equal(count('ws'), 2);
  assert.equal(roster.filter((a) => p.get(a.key).kind === 'stand').length, 1);
  assert.equal(new Set([...p.values()].filter((v) => v.kind === 'seat').map((v) => v.x + ',' + v.z)).size, 4);
});

test('missing room types fall back to workspace, then lobby', () => {
  const rooms = [room('lobby', 'lobby', 0, 0), room('ws', 'workspace', 12, 0)];
  const l = office(rooms, desks(rooms[1], 3));
  const p = planSeats(l, [main('m'), sub('e', 'Explore'), sub('r', 'reviewer'), sub('p', 'Plan')], CATALOG);
  assert.equal(p.get('m').room, 'ws');
  assert.equal(p.get('e').room, 'ws');
  assert.equal(p.get('p').room, 'ws');
  // three desks for four agents: the last in key order stands in the lobby
  assert.equal(p.get('r').kind, 'stand');
  assert.equal(p.get('r').room, 'lobby');
});

test('no workspace and no lobby still places everyone', () => {
  const l = office([], []);
  const p = planSeats(l, [main('m'), sub('e', 'Explore')], CATALOG);
  assert.equal(p.size, 2);
  assert.equal(p.get('m').kind, 'stand');
  assert.equal(p.get('m').room, null);
});

test('standing agents get distinct lobby cells clear of furniture', () => {
  const l = office([room('lobby', 'lobby', 0, 0)], [{ id: 'pl', item: 'plant', level: 0, x: 0, z: 0, rot: 0, room: 'lobby' }]);
  const roster = Array.from({ length: 20 }, (_, i) => main('m' + i));
  const p = planSeats(l, roster, CATALOG);
  const cells = new Set([...p.values()].map((v) => v.x + ',' + v.z));
  assert.equal(cells.size, 20);
  assert.ok(!cells.has('0.5,0.5'));
  for (const v of p.values()) { assert.ok(v.x >= 0 && v.x <= 10 && v.z >= 0 && v.z <= 6); }
});

test('seat stability across calls and roster order', () => {
  const l = full();
  const roster = [sub('a', 'Explore'), sub('b', 'Explore'), sub('c', 'Explore')];
  const p1 = planSeats(l, roster, CATALOG);
  const p2 = planSeats(l, [sub('x', 'Explore'), ...roster.slice().reverse()], CATALOG, p1);
  for (const k of ['a', 'b', 'c']) assert.deepEqual(p2.get(k), p1.get(k), k);
  // agent leaves, others keep their seat
  const p3 = planSeats(l, [roster[2]], CATALOG, p2);
  assert.deepEqual(p3.get('c'), p1.get('c'));
});

test('a standing agent takes a freed seat but seated ones do not move', () => {
  const rooms = [room('lobby', 'lobby', 0, 0), room('res', 'research', 12, 0)];
  const l = office(rooms, desks(rooms[1], 1));
  const p1 = planSeats(l, [sub('a', 'Explore'), sub('b', 'Explore')], CATALOG);
  assert.equal(p1.get('b').kind, 'stand');
  const p2 = planSeats(l, [sub('b', 'Explore')], CATALOG, p1);
  assert.equal(p2.get('b').kind, 'seat');
});

test('a previous seat that no longer exists is replaced', () => {
  const l = full();
  const p1 = planSeats(l, [sub('a', 'Explore')], CATALOG);
  const gone = l.furniture.filter((f) => f.room !== 'res' || f.id !== p1.get('a').seat);
  const p2 = planSeats({ ...l, furniture: gone }, [sub('a', 'Explore')], CATALOG, p1);
  assert.equal(p2.get('a').room, 'res');
  assert.notEqual(p2.get('a').seat, p1.get('a').seat);
});

test('deterministic for the same inputs', () => {
  const l = full();
  const roster = Array.from({ length: 30 }, (_, i) => sub('s' + i, i % 2 ? 'Explore' : 'Plan'));
  assert.deepEqual([...planSeats(l, roster, CATALOG)], [...planSeats(l, roster, CATALOG)]);
});

test('100 agents stay fast and never share a seat', () => {
  const l = full();
  const roster = Array.from({ length: 100 }, (_, i) => (i % 5 === 0 ? main('m' + i) : sub('s' + i, ['Explore', 'Plan', 'reviewer', 'x'][i % 4])));
  const t = performance.now();
  let p = planSeats(l, roster, CATALOG);
  for (let i = 0; i < 20; i++) p = planSeats(l, roster, CATALOG, p);
  assert.ok(performance.now() - t < 1000, 'too slow');
  assert.equal(p.size, 100);
  const seats = [...p.values()].filter((v) => v.kind === 'seat').map((v) => v.seat);
  assert.equal(new Set(seats).size, seats.length);
});

test('deck level rooms route and keep their level', () => {
  const lobby = room('lobby', 'lobby', 0, 0);
  const rev = room('rev', 'review', 30, 0, { level: 1 });
  const l = office([lobby, rev], desks(rev, 2));
  l.deck = { x: 30, z: 0, w: 16, d: 8 };
  const p = planSeats(l, [sub('r', 'reviewer')], CATALOG);
  assert.equal(p.get('r').level, 1);
  assert.equal(p.get('r').room, 'rev');
});

test('a level 1 room does not claim level 0 furniture at the same cell', () => {
  const lobby = room('lobby', 'lobby', 0, 0);
  const rev = room('rev', 'review', 0, 0, { level: 1 });
  const l = office([lobby, rev], [{ id: 'a', item: 'desk', level: 0, x: 1, z: 1, rot: 0 }]);
  assert.equal(planSeats(l, [sub('r', 'reviewer')], CATALOG).get('r').kind, 'stand');
});

test('break spots come from cafe and play seats and play items', () => {
  const l = full();
  const s = breakSpots(l, CATALOG);
  assert.equal(s.cafe.length, 4);
  assert.equal(s.play.length, 4);
  assert.ok(s.cafe.every((x) => x.room === 'cafe' && x.level === 0));
  assert.equal(s.all.length, 8);
});

test('break spots without cafe or play fall back to the lobby', () => {
  const l = office([room('lobby', 'lobby', 0, 0)], []);
  const s = breakSpots(l, CATALOG);
  assert.equal(s.cafe.length, 0);
  assert.equal(s.play.length, 0);
  assert.ok(s.all.length > 0);
  assert.ok(s.all.every((x) => x.room === 'lobby'));
});

test('break spots with only a cafe use just the cafe', () => {
  const rooms = [room('lobby', 'lobby', 0, 0), room('cafe', 'cafe', 12, 0)];
  const l = office(rooms, desks(rooms[1], 2, 'sofa'));
  const s = breakSpots(l, CATALOG);
  assert.equal(s.cafe.length, 2);
  assert.equal(s.play.length, 0);
  assert.equal(s.all.length, 2);
});

test('a play room with no tagged furniture offers free floor cells', () => {
  const rooms = [room('lobby', 'lobby', 0, 0), room('play', 'play', 12, 0)];
  const l = office(rooms, []);
  const s = breakSpots(l, CATALOG);
  assert.ok(s.play.length > 0);
  assert.ok(s.play.every((x) => x.x >= 12 && x.x <= 22));
});

test('break spots with no rooms at all return an empty list', () => {
  const s = breakSpots(office([], []), CATALOG);
  assert.deepEqual(s, { cafe: [], play: [], all: [] });
});

test('missing catalog warns once and falls back to furniture tags', (ctx) => {
  const warn = ctx.mock.method(console, 'warn', () => {});
  const l = office([room('lobby', 'lobby', 0, 0), room('ws', 'workspace', 12, 0)],
    [{ id: 'a', item: 'x', tags: ['seat'], level: 0, x: 13, z: 2, rot: 0, room: 'ws' }]);
  planSeats(l, [sub('s1', 'x')]);
  planSeats(l, [sub('s1', 'x')]);
  breakSpots(l);
  assert.equal(warn.mock.callCount(), 1);
});

test('a duplicate roster key cannot leave a seat claimed', () => {
  const rooms = [room('lobby', 'lobby', 0, 0), room('ws', 'workspace', 12, 0)];
  const l = office(rooms, desks(rooms[1], 2));
  const p = planSeats(l, [sub('a', 'x'), sub('a', 'Explore'), sub('b', 'x')], CATALOG);
  assert.equal(p.size, 2);
  assert.equal(p.get('a').kind, 'seat');
  assert.equal(p.get('b').kind, 'seat');
});
