// shared cabins in the hall are handed out in one contiguous block per account
import test from 'node:test';
import assert from 'node:assert/strict';
import { planSeats } from '../routing.js';

const CATALOG = [{ id: 'desk', footprint: { w: 2, d: 1 }, tags: ['seat'] }];
let fid = 0;
const room = (id, type, x, z, w = 24, d = 8, account) => ({ id, type, level: 0, x, z, w, d, label: id, ...(account ? { account } : {}) });
const desksIn = (r, n) => Array.from({ length: n }, (_, i) => ({ id: 'f' + (fid++), item: 'desk', level: 0, x: r.x + 1 + i * 3, z: r.z + 1, rot: 0, room: r.id }));

// cabins room with n desks in a row, an optional account cabins room, workspace and lobby
function hall({ shared = 6, owned = 0, account = 'client', ws = 4 } = {}) {
  const rooms = [room('lobby', 'lobby', 0, 0, 8, 8), room('cab', 'cabins', 0, 10, 24), room('ws', 'workspace', 0, 20, 24), room('cafe', 'cafe', 30, 0, 10)];
  if (owned) rooms.push(room('own', 'cabins', 0, 30, 24, 8, account));
  const furniture = [...desksIn(rooms[1], shared), ...desksIn(rooms[2], ws), ...(owned ? desksIn(rooms[rooms.length - 1], owned) : [])];
  return { v: 1, name: 't', grid: 0.5, size: { w: 60, d: 50 }, door: { side: 'left', at: 2 }, deck: null, stairs: null, rooms, furniture };
}
const main = (key, account, state = 'busy') => ({ key, kind: 'main', account, type: 'main', state });
const plan = (l, roster, prev) => planSeats(l, roster, CATALOG, prev);
const accountsByX = (p, roster) => roster.filter((a) => a.kind === 'main' && p.get(a.key).room === 'cab')
  .sort((a, b) => p.get(a.key).x - p.get(b.key).x).map((a) => a.account).join(' ');

const MIXED = [main('s1', 'work'), main('s2', 'personal'), main('s3', 'client'), main('s4', 'work'), main('s5', 'personal'), main('s6', 'work')];

test('mixed roster: each account gets a contiguous block, accounts in id order', () => {
  const p = plan(hall(), MIXED);
  assert.equal(accountsByX(p, MIXED), 'client personal personal work work work');
});

test('an idle session lands inside its account block', () => {
  const roster = [main('s1', 'work'), main('s2', 'personal'), main('s3', 'client', 'idle'), main('s4', 'work', 'idle'), main('s5', 'personal'), main('s6', 'work')];
  const p = plan(hall(), roster);
  assert.equal(accountsByX(p, roster), 'client personal personal work work work');
});

test('a scattered previous plan regroups', () => {
  const l = hall();
  const ids = l.furniture.filter((f) => f.room === 'cab').map((f) => f.id);
  // work, personal, client, work, personal, work: the interleaved order a fifo plan would leave
  const prev = new Map(MIXED.map((a, i) => [a.key, { kind: 'seat', level: 0, x: 1 + i * 3, z: 11, rot: 0, room: 'cab', seat: ids[i] }]));
  const p = plan(l, MIXED, prev);
  assert.equal(accountsByX(p, MIXED), 'client personal personal work work work');
});

test('a seat inside the block is kept between polls', () => {
  const l = hall();
  const p1 = plan(l, MIXED);
  const p2 = plan(l, MIXED, p1);
  for (const a of MIXED) assert.deepEqual(p2.get(a.key), p1.get(a.key), a.key);
});

test('a room owned by the account takes that account first', () => {
  const roster = [main('s1', 'work'), main('s2', 'client'), main('s3', 'personal')];
  const p = plan(hall({ owned: 2, account: 'client' }), roster);
  assert.equal(p.get('s2').room, 'own');
  assert.equal(p.get('s1').room, 'cab');
  assert.equal(p.get('s3').room, 'cab');
});

test('more mains than cabins: the overflow goes to the workspace', () => {
  const roster = [main('s1', 'work'), main('s2', 'personal'), main('s3', 'client'), main('s4', 'work'), main('s5', 'personal')];
  const p = plan(hall({ shared: 3 }), roster);
  const inCab = roster.filter((a) => p.get(a.key).room === 'cab');
  const inWs = roster.filter((a) => p.get(a.key).room === 'ws');
  assert.equal(inCab.length, 3);
  assert.equal(inWs.length, 2);
});

test('team, intern and sub seating ignores the blocks', () => {
  const roster = [...MIXED,
    { key: 'team:a', kind: 'team', team: 'a', type: 'a', state: 'busy' },
    { key: 's1/i', kind: 'intern', team: 'a', owner: 's1', account: 'work', type: 'a', state: 'busy' },
    { key: 's1/x', kind: 'sub', account: 'work', type: 'general-purpose', state: 'busy' }];
  const p = plan(hall(), roster);
  for (const k of ['team:a', 's1/i', 's1/x']) assert.equal(p.get(k).room, 'ws', k);
});

test('sessions that fit in their own room reserve no shared cabins', () => {
  // client has two own cabins for its two sessions, so the shared strip starts with personal
  const roster = [main('s1', 'client'), main('s2', 'client'), main('s3', 'personal'), main('s4', 'work')];
  const p = plan(hall({ owned: 2, account: 'client' }), roster);
  const l = hall({ owned: 2, account: 'client' });
  const first = l.furniture.filter((f) => f.room === 'cab').sort((a, b) => a.x - b.x)[0];
  assert.equal(p.get('s3').x, first.x + 1);
  assert.equal(accountsByX(p, roster), 'personal work');
});
