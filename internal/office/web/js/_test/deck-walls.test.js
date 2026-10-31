// rooms that meet on the deck get a glass wall with a doorway at the front end
import test from 'node:test';
import assert from 'node:assert/strict';
import { deckWalls } from '../scene/partitions.js';

const room = (id, x, z, w, d, level = 1) => ({ id, type: 'meeting', level, x, z, w, d });

test('two rooms side by side on the deck share one wall with a doorway at the front', () => {
  const { walls, closed } = deckWalls({ rooms: [room('review', 28, 1, 8, 8), room('meeting', 36, 1, 8, 8)] });
  assert.deepEqual(walls, [{ axis: 'x', at: 36, from: 1, to: 7 }]);
  // the cells just inside the left room along the wall, without the two doorway cells
  assert.deepEqual(closed, [1, 2, 3, 4, 5, 6].map((z) => ({ x: 35, z })));
});

test('rooms one behind the other share a wall along x with the doorway at the right end', () => {
  const { walls, closed } = deckWalls({ rooms: [room('a', 0, 0, 6, 4), room('b', 0, 4, 6, 4)] });
  assert.deepEqual(walls, [{ axis: 'z', at: 4, from: 0, to: 4 }]);
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
