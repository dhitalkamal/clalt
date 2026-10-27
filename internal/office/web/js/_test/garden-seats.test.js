// seats, overflow and the local layouts of pavilion contents
import test from 'node:test';
import assert from 'node:assert/strict';
import { fixture, NAMES, ALL } from './garden-fixtures.js';
import { gardenSeats, gardenOverflow, MEZ } from '../garden/seats.js';
import { inPoly } from '../garden/geo.js';

const count = (seats, kind) => seats.filter((s) => s.kind === kind).length;

test('river layout has the expected seats per kind', () => {
  const L = fixture('river');
  const seats = gardenSeats(L);
  // 3 cabins x 2 desks, workspace 4 cols x 2 lines, research 2 cols x 2 lines
  assert.equal(count(seats, 'desk'), 6 + 8 + 4);
  assert.equal(count(seats, 'review'), 3);
  assert.equal(count(seats, 'meeting'), 3);
  assert.equal(count(seats, 'cafe'), 6);
  assert.equal(count(seats, 'play'), 5);
  assert.ok(count(seats, 'lobby') >= 3);
});

test('seat ids are unique and carry pavilion and account', () => {
  for (const name of ALL) {
    const seats = gardenSeats(fixture(name));
    assert.equal(new Set(seats.map((s) => s.id)).size, seats.length, name);
    const cabin = seats.find((s) => s.type === 'cabin');
    assert.ok(cabin.account && cabin.pavilionId.startsWith('cabin-'));
    for (const s of seats) {
      assert.ok(Number.isFinite(s.x) && Number.isFinite(s.z) && Number.isFinite(s.rot), s.id);
      assert.ok(['desk', 'cafe', 'play', 'review', 'meeting', 'lobby', 'stool'].includes(s.kind));
    }
  }
});

test('treehouse seats stand on the raised deck, the rest on the ground', () => {
  const seats = gardenSeats(fixture('rows'));
  for (const s of seats) assert.equal(s.y, s.kind === 'review' || s.kind === 'meeting' ? MEZ : 0, s.id);
});

test('every seat lies on the lawn', () => {
  for (const name of ALL) {
    const L = fixture(name);
    for (const s of gardenSeats(L)) {
      assert.ok(L.site.land.some((l) => inPoly(s.x, s.z, l.poly)), `${name} ${s.id} at ${s.x},${s.z}`);
    }
  }
});

test('a rotated pavilion moves its desks with it', () => {
  const L = fixture('rows');
  const cab = L.pavilions.find((p) => p.type === 'cabin');
  const before = gardenSeats(L).filter((s) => s.pavilionId === cab.id);
  cab.rot = 90;
  const after = gardenSeats(L).filter((s) => s.pavilionId === cab.id);
  assert.equal(before.length, after.length);
  // desk 0 and 1 sit side by side along x at rot 0 and along z at rot 90
  assert.ok(Math.abs(before[0].x - before[1].x) > 1.5 && Math.abs(before[0].z - before[1].z) < 0.01);
  assert.ok(Math.abs(after[0].z - after[1].z) > 1.5 && Math.abs(after[0].x - after[1].x) < 0.01);
  assert.equal(after[0].rot, 90);
});

test('open desk pavilions seat cols x rows x 2, the two lines face each other', () => {
  const L = fixture('river');
  const ws = L.pavilions.find((p) => p.type === 'workspace');
  ws.rot = 0;
  const seats = gardenSeats(L).filter((s) => s.pavilionId === ws.id);
  assert.equal(seats.length, ws.cols * ws.rows * 2);
  const north = seats.filter((s) => s.rot === 0);
  const south = seats.filter((s) => s.rot === 180);
  assert.equal(north.length, ws.cols);
  assert.equal(south.length, ws.cols);
  for (const n of north) assert.ok(south.some((s) => Math.abs(s.x - n.x) < 1e-9 && s.z > n.z + 0.5), 'a desk faces each desk');
  // cabins keep one line
  const cab = gardenSeats(L).filter((s) => s.type === 'cabin');
  assert.ok(cab.every((s) => s.rot === 0));
  // two rows double again
  ws.rows = 2;
  assert.equal(gardenSeats(L).filter((s) => s.pavilionId === ws.id).length, ws.cols * 4);
});

test('desk seats name the account of their cabin', () => {
  const seats = gardenSeats(fixture('village')).filter((s) => s.type === 'cabin');
  assert.deepEqual([...new Set(seats.map((s) => s.account))].sort(), ['client', 'personal', 'work']);
});

test('overflow spots come back in world coordinates with the near hint', () => {
  const L = fixture('river');
  L.overflow = [{ x: 1, z: 2, rot: 90, near: 'cafe' }, { x: 3, z: 4, rot: 0, near: 'lobby' }];
  const o = gardenOverflow(L);
  assert.equal(o.length, 2);
  assert.deepEqual([o[0].x, o[0].z, o[0].rot, o[0].near, o[0].y], [1, 2, 90, 'cafe', 0]);
  assert.equal(gardenOverflow({ ...L, overflow: undefined }).length, 0);
});

test('fixture overflow spots sit on the lawn', () => {
  for (const name of ALL) {
    const L = fixture(name);
    const o = gardenOverflow(L);
    assert.ok(o.length >= 4, name);
    for (const s of o) assert.ok(L.site.land.some((l) => inPoly(s.x, s.z, l.poly)), `${name} ${s.id}`);
  }
});
