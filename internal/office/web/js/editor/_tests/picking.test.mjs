import test from 'node:test';
import assert from 'node:assert/strict';
import * as p from '../picking.js';
import { catalog, baseLayout } from './fixtures.mjs';

const cat = new Map(catalog.map((c) => [c.id, c]));

test('hitFurniture finds the item under a cell on the level', () => {
  const l = baseLayout();
  assert.equal(p.hitFurniture(l, cat, 0, 12, 2).id, 'f1');
  assert.equal(p.hitFurniture(l, cat, 0, 12, 3).id, 'f2');
  assert.equal(p.hitFurniture(l, cat, 0, 30, 10), null);
  assert.equal(p.hitFurniture(l, cat, 1, 12, 2), null);
});

test('hitRoom finds the room under a cell on the level', () => {
  const l = baseLayout();
  assert.equal(p.hitRoom(l, 0, 3, 3).id, 'r1');
  assert.equal(p.hitRoom(l, 0, 9, 3), null);
  assert.equal(p.hitRoom(l, 1, 21, 2).id, 'r3');
});

test('placeAnchor centers the footprint on the cursor cell', () => {
  assert.deepEqual(p.placeAnchor(cat, 'desk', 0, 10, 10), { x: 9, z: 9 });
  assert.deepEqual(p.placeAnchor(cat, 'desk', 90, 10, 10), { x: 9, z: 9 });
  assert.equal(p.placeAnchor(cat, 'ghost', 0, 1, 1), null);
});

test('canPlace rejects out of bounds and overlapping spots', () => {
  const l = baseLayout();
  assert.equal(p.canPlace(l, cat, 'chair', 0, 0, 14, 5, 0), true);
  assert.equal(p.canPlace(l, cat, 'chair', 0, 0, 12, 1, 0), false);
  assert.equal(p.canPlace(l, cat, 'sofa', 0, 0, 38, 5, 0), false);
});

test('canPlace lets a floor item share cells both ways', () => {
  const l = baseLayout();
  const rugCat = new Map([...cat, ['rug', { id: 'rug', footprint: { w: 4, d: 3 }, levels: [0], tags: ['floor'] }]]);
  assert.equal(p.canPlace(l, rugCat, 'rug', 0, 0, 10, 1, 0), true);
  l.furniture.push({ id: 'r', item: 'rug', level: 0, x: 20, z: 10, rot: 0 });
  assert.equal(p.canPlace(l, rugCat, 'chair', 0, 0, 21, 11, 0), true);
});
