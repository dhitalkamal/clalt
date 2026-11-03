// rooms that meet on the deck get walls: a solid one between them, and a glass front with a door when a corridor
// runs in front of them
import test from 'node:test';
import assert from 'node:assert/strict';
import { deckWalls, wallParts } from '../scene/partitions.js';
import { MEZ } from '../scene/geom.js';
import { hallCompiled } from './hall-fixture.js';

const room = (id, x, z, w, d, level = 1) => ({ id, type: 'meeting', level, x, z, w, d });
const deck = { x: 28, z: 1, w: 16, d: 8 };

test('two rooms side by side reaching the deck front share one wall with a doorway at the front', () => {
  const { walls, closed } = deckWalls({ deck, rooms: [room('review', 28, 1, 8, 8), room('meeting', 36, 1, 8, 8)] });
  assert.deepEqual(walls, [{ axis: 'x', at: 36, from: 1, to: 9, door: [7, 9], look: 'solid' }]);
  // the cells just inside the left room along the wall, without the two doorway cells
  assert.deepEqual(closed, [1, 2, 3, 4, 5, 6].map((z) => ({ x: 35, z })));
});

test('rooms one behind the other share a wall along x with the doorway at the right end', () => {
  const { walls, closed } = deckWalls({ rooms: [room('a', 0, 0, 6, 4), room('b', 0, 4, 6, 4)] });
  assert.deepEqual(walls, [{ axis: 'z', at: 4, from: 0, to: 6, door: [4, 6], look: 'solid' }]);
  assert.deepEqual(closed, [0, 1, 2, 3].map((x) => ({ x, z: 3 })));
});

test('ground rooms, rooms that do not touch and a single deck room get no wall', () => {
  assert.deepEqual(deckWalls({ rooms: [room('a', 0, 0, 4, 4, 0), room('b', 4, 0, 4, 4, 0)] }).walls, []);
  assert.deepEqual(deckWalls({ rooms: [room('a', 0, 0, 4, 4), room('b', 6, 0, 4, 4)] }).walls, []);
  assert.deepEqual(deckWalls({ rooms: [room('a', 0, 0, 4, 4)] }).walls, []);
  assert.deepEqual(deckWalls({}).walls, []);
});

test('a shared edge too short for a doorway stays fully open', () => {
  assert.deepEqual(deckWalls({ rooms: [room('a', 0, 0, 4, 2), room('b', 4, 0, 4, 2)] }).walls, []);
});

test('rooms with a corridor in front get a solid wall between them and a glass front each, door in the middle', () => {
  const { walls, closed } = deckWalls({ deck, rooms: [room('review', 28, 1, 8, 6), room('meeting', 36, 1, 8, 6)] });
  assert.deepEqual(walls, [
    { axis: 'x', at: 36, from: 1, to: 7, door: null, look: 'solid' },
    { axis: 'z', at: 7, from: 28, to: 36, door: [31, 33], look: 'glass' },
    { axis: 'z', at: 7, from: 36, to: 44, door: [39, 41], look: 'glass' },
  ]);
  const key = (c) => c.x + ',' + c.z;
  const got = new Set(closed.map(key));
  // the whole shared column inside the left room, and the corridor row in front of both rooms but the doors
  for (let z = 1; z < 7; z++) assert.ok(got.has('35,' + z), 'shared wall closes 35,' + z);
  for (let x = 28; x < 44; x++) {
    const door = (x >= 31 && x < 33) || (x >= 39 && x < 41);
    assert.equal(got.has(x + ',7'), !door, 'corridor cell ' + x + ',7');
  }
});

test('a front door moves off furniture to the nearest free gap', () => {
  // furniture right behind the middle of the left room's front
  const free = (x, z) => !(z === 6 && (x === 31 || x === 32));
  const { walls } = deckWalls({ deck, rooms: [room('review', 28, 1, 8, 6), room('meeting', 36, 1, 8, 6)] }, free);
  assert.deepEqual(walls[1].door, [29, 31]);
});

test('a deck wall is solid to the full storey height with a lintel over its doorway', () => {
  const parts = wallParts({ axis: 'x', at: 36, from: 1, to: 9, door: [7, 9], look: 'solid' });
  assert.deepEqual(parts.map(({ from, to, kind }) => ({ from, to, kind })), [
    { from: 1, to: 7, kind: 'wall' },
    { from: 7, to: 9, kind: 'lintel' },
  ]);
  for (const p of parts) assert.equal(p.y1, MEZ);
  assert.equal(parts[0].y0, 0);
  assert.ok(parts[1].y0 > 0.85 && parts[1].y0 < MEZ, 'the doorway stays tall enough to walk through');
});

test('a door in the middle of a wall leaves wall on both sides of it', () => {
  const parts = wallParts({ axis: 'z', at: 7, from: 28, to: 36, door: [31, 33], look: 'glass' });
  assert.deepEqual(parts.map(({ from, to, kind }) => [from, to, kind]), [[28, 31, 'wall'], [33, 36, 'wall'], [31, 33, 'lintel']]);
  assert.deepEqual(wallParts({ axis: 'x', at: 36, from: 1, to: 7, door: null, look: 'solid' }).map((p) => [p.from, p.to]), [[1, 7]]);
});

test('in the hall nobody walks through one upstairs room to reach the other', () => {
  const L = hallCompiled();
  const up = L.layout.rooms.filter((r) => r.level === 1);
  const roomAt = (p) => { const c = L.cellOf(p); return up.find((r) => c.x >= r.x && c.x < r.x + r.w && c.z >= r.z && c.z < r.z + r.d); };
  const seats = L.items.filter((it) => it.level === 1 && L.seatSlot(it));
  assert.ok(seats.length > 0);
  for (const it of seats) {
    const home = roomAt(L.seatSlot(it));
    assert.ok(home, it.id + ' sits in an upstairs room');
    const pts = L.path(L.spawn, L.seatSlot(it).node).filter((p) => p.y > 1);
    // sample every leg so a straight cut across a room counts too
    for (let i = 0; i + 1 < pts.length; i++) {
      const [p, q] = [pts[i], pts[i + 1]];
      const n = Math.max(1, Math.ceil(Math.hypot(q.x - p.x, q.z - p.z) / 0.05));
      for (let k = 0; k <= n; k++) {
        const r = roomAt({ x: p.x + (q.x - p.x) * k / n, z: p.z + (q.z - p.z) * k / n });
        assert.ok(!r || r.id === home.id, it.id + ' in ' + home.id + ' walks through ' + (r && r.id));
      }
    }
  }
});
