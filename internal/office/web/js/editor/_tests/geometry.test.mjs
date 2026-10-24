import test from 'node:test';
import assert from 'node:assert/strict';
import * as g from '../geometry.js';
import { catalog, baseLayout } from './fixtures.mjs';

const cat = new Map(catalog.map((c) => [c.id, c]));

test('rotateFootprint swaps w and d on quarter turns', () => {
  assert.deepEqual(g.rotateFootprint({ w: 3, d: 2 }, 0), { w: 3, d: 2 });
  assert.deepEqual(g.rotateFootprint({ w: 3, d: 2 }, 90), { w: 2, d: 3 });
  assert.deepEqual(g.rotateFootprint({ w: 3, d: 2 }, 180), { w: 3, d: 2 });
  assert.deepEqual(g.rotateFootprint({ w: 3, d: 2 }, 270), { w: 2, d: 3 });
});

test('nextRot cycles through four steps', () => {
  assert.equal(g.nextRot(0), 90);
  assert.equal(g.nextRot(270), 0);
});

test('rectsOverlap treats touching edges as clear', () => {
  const a = { x: 0, z: 0, w: 4, d: 4 };
  assert.equal(g.rectsOverlap(a, { x: 4, z: 0, w: 2, d: 2 }), false);
  assert.equal(g.rectsOverlap(a, { x: 3, z: 3, w: 2, d: 2 }), true);
});

test('rectInside needs the whole rect within bounds', () => {
  const o = { x: 0, z: 0, w: 10, d: 10 };
  assert.equal(g.rectInside({ x: 8, z: 8, w: 2, d: 2 }, o), true);
  assert.equal(g.rectInside({ x: 9, z: 8, w: 2, d: 2 }, o), false);
  assert.equal(g.rectInside({ x: -1, z: 0, w: 2, d: 2 }, o), false);
});

test('rectFromPoints is inclusive and order independent', () => {
  assert.deepEqual(g.rectFromPoints(2, 3, 5, 4), { x: 2, z: 3, w: 4, d: 2 });
  assert.deepEqual(g.rectFromPoints(5, 4, 2, 3), { x: 2, z: 3, w: 4, d: 2 });
  assert.deepEqual(g.rectFromPoints(2, 2, 2, 2), { x: 2, z: 2, w: 1, d: 1 });
});

test('resizeRect moves the dragged edges only', () => {
  const r = { x: 4, z: 4, w: 6, d: 6 };
  assert.deepEqual(g.resizeRect(r, 'e', 2, 9, 2), { x: 4, z: 4, w: 8, d: 6 });
  assert.deepEqual(g.resizeRect(r, 'w', -2, 0, 2), { x: 2, z: 4, w: 8, d: 6 });
  assert.deepEqual(g.resizeRect(r, 'nw', 1, 1, 2), { x: 5, z: 5, w: 5, d: 5 });
  assert.deepEqual(g.resizeRect(r, 'se', 1, -1, 2), { x: 4, z: 4, w: 7, d: 5 });
});

test('resizeRect never goes below the minimum size', () => {
  const r = { x: 4, z: 4, w: 6, d: 6 };
  assert.deepEqual(g.resizeRect(r, 'e', -20, 0, 2), { x: 4, z: 4, w: 2, d: 6 });
  assert.deepEqual(g.resizeRect(r, 'w', 20, 0, 2), { x: 8, z: 4, w: 2, d: 6 });
});

test('furnitureRect uses the rotated catalog footprint', () => {
  const f = { item: 'desk', x: 5, z: 5, rot: 90 };
  assert.deepEqual(g.furnitureRect(f, cat), { x: 5, z: 5, w: 2, d: 3 });
  assert.equal(g.furnitureRect({ item: 'nope', x: 0, z: 0, rot: 0 }, cat), null);
});

test('levelBounds is the floor on 0, the deck on 1, null without deck', () => {
  const l = baseLayout();
  assert.deepEqual(g.levelBounds(l, 0), { x: 0, z: 0, w: 40, d: 20 });
  assert.deepEqual(g.levelBounds(l, 1), l.deck);
  l.deck = null;
  assert.equal(g.levelBounds(l, 1), null);
});

test('nearestDoor snaps to the closest side and offset', () => {
  const l = baseLayout();
  assert.deepEqual(g.nearestDoor(l, 0, 7), { side: 'left', at: 7 });
  assert.deepEqual(g.nearestDoor(l, 12, 19), { side: 'front', at: 12 });
  assert.deepEqual(g.nearestDoor(l, 39, 3), { side: 'right', at: 3 });
  assert.deepEqual(g.nearestDoor(l, 5, 0), { side: 'back', at: 5 });
});

test('pointInRect is half open', () => {
  const r = { x: 2, z: 2, w: 2, d: 2 };
  assert.equal(g.pointInRect(2, 2, r), true);
  assert.equal(g.pointInRect(4, 2, r), false);
});
