// resting in a chosen room: a seat-tagged spot first, else a free floor cell, else nothing (the caller takes a break)
import test from 'node:test';
import assert from 'node:assert/strict';
import { restPlan } from '../idle/rest.js';
import { makeGrid, setOpen } from '../scene/nav.js';

// a 10x10 hall: room r1 is x 0..3, z 0..3
function hall(extra = {}) {
  const g = makeGrid(10, 10);
  g.open.fill(1);
  const L = {
    rooms: [{ id: 'r1', level: 0, x: 0, z: 0, w: 4, d: 4 }, { id: 'r2', level: 0, x: 5, z: 0, w: 2, d: 2 }],
    grids: [g, makeGrid(10, 10)], spots: {}, groups: {},
    X: (c) => c - 5, Z: (c) => c - 5, node: (level, p) => ({ level, ...p }),
    flood: { has: () => true }, ...extra,
  };
  return { L, g };
}

test('a free floor cell of the room becomes one long rest step', () => {
  const { L } = hall();
  const plan = restPlan(L, 'r1', new Set(), 3);
  assert.equal(plan.steps.length, 1);
  assert.equal(plan.steps[0].act, 'rest');
  assert.ok(plan.steps[0].dur > 1e6);
  const spot = L.spots[plan.steps[0].spot];
  assert.ok(spot.x >= -5 && spot.x <= -1 && spot.z >= -5 && spot.z <= -1, 'inside room r1');
  assert.equal(spot.sit, false);
});

test('a seat-tagged spot in the room is used before the floor', () => {
  const { L } = hall();
  const bag = { name: 'bag1', x: -4, z: -4, yaw: 0, act: 'read', sit: true, level: 0, item: { room: { id: 'r1' } }, node: {} };
  L.spots.bag1 = bag;
  const plan = restPlan(L, 'r1', new Set(), 0);
  assert.equal(plan.steps[0].spot, 'bag1');
});

test('reserved spots are skipped and two agents get different places', () => {
  const { L } = hall();
  const taken = new Set();
  const a = restPlan(L, 'r1', taken, 1);
  taken.add(a.steps[0].spot);
  const b = restPlan(L, 'r1', taken, 1);
  assert.notEqual(a.steps[0].spot, b.steps[0].spot);
});

test('cells under furniture and cells nobody can reach are not used', () => {
  const { L, g } = hall();
  for (let x = 0; x < 4; x++) for (let z = 0; z < 4; z++) setOpen(g, x, z, false);
  assert.equal(restPlan(L, 'r1', new Set(), 0), null);
  const open = hall({ flood: { has: () => false } });
  assert.equal(restPlan(open.L, 'r1', new Set(), 0), null);
});

test('a room the layout does not have gives nothing, so does a layout without rooms', () => {
  const { L } = hall();
  assert.equal(restPlan(L, 'nope', new Set(), 0), null);
  assert.equal(restPlan({ spots: {}, groups: {} }, 'r1', new Set(), 0), null);
});
