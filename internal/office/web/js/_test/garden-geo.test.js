// pure geometry of the garden renderer: footprints, rotation, polygon helpers
import test from 'node:test';
import assert from 'node:assert/strict';
import { inPoly, distLine, footprint, polysOverlap, curve, boundsOf, along } from '../garden/geo.js';
import { sizeOf, polyOf, toWorld, deskCells, FOOTPRINTS } from '../garden/footprints.js';

const near = (a, b, eps = 1e-6) => assert.ok(Math.abs(a - b) < eps, `${a} vs ${b}`);

test('sizeOf follows the shared footprint table', () => {
  assert.deepEqual(sizeOf({ type: 'cabin', cols: 2, rows: 1 }), { w: 4.2, d: 2.8 });
  const ws = sizeOf({ type: 'workspace', cols: 4, rows: 2 });
  near(ws.w, 2.1 * 4 + 0.2);
  near(ws.d, 3.5 * 2 + 0.2);
  assert.deepEqual(sizeOf({ type: 'treehouse' }), { w: 9.6, d: 4.2 });
  assert.equal(sizeOf({ type: 'nope' }), null);
  assert.equal(FOOTPRINTS.lobby.w, 3.8);
});

test('toWorld turns a local point by rot degrees, 0 faces south', () => {
  const p = { x: 10, z: 5, rot: 0 };
  assert.deepEqual(toWorld(p, 1, 2).map((v) => Math.round(v * 1e6) / 1e6), [11, 7]);
  // 90 degrees counterclockwise seen from above: local north (-z) points west
  const q = toWorld({ x: 10, z: 5, rot: 90 }, 0, -2);
  near(q[0], 8);
  near(q[1], 5);
});

test('polyOf gives the rotated corners and they overlap test works', () => {
  const a = polyOf({ type: 'lobby', x: 0, z: 0, rot: 0 });
  assert.equal(a.length, 4);
  const b = polyOf({ type: 'lobby', x: 3, z: 0, rot: 0 });
  assert.ok(polysOverlap(a, b));
  const c = polyOf({ type: 'lobby', x: 4, z: 0, rot: 0 });
  assert.ok(!polysOverlap(a, c));
});

test('deskCells lists cols x rows cells centred in the pavilion', () => {
  const cells = deskCells({ type: 'cabin', cols: 2, rows: 1 });
  assert.equal(cells.length, 2);
  near(cells[0].lx, -1.05);
  near(cells[1].lx, 1.05);
  // open pavilions hold two face-to-face lines in every row
  const big = deskCells({ type: 'workspace', cols: 3, rows: 2 });
  assert.equal(big.length, 12);
  assert.ok(big.every((c) => Math.abs(c.lx) < 3.2 && Math.abs(c.lz) < 3.6));
});

test('polygon helpers', () => {
  const sq = [[0, 0], [4, 0], [4, 4], [0, 4]];
  assert.ok(inPoly(2, 2, sq));
  assert.ok(!inPoly(5, 2, sq));
  near(distLine(2, 3, [[0, 0], [4, 0]]), 3);
  assert.deepEqual(boundsOf(sq), { x0: 0, x1: 4, z0: 0, z1: 4 });
  assert.equal(footprint(0, 0, 2, 2, 0).length, 4);
  const c = curve([[0, 0], [2, 0], [4, 2]], 4);
  assert.deepEqual(c[c.length - 1], [4, 2]);
  near(along([[0, 0], [10, 0]], 0.5).x, 5);
});
