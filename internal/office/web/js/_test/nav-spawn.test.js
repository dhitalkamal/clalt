// the ground spawn: nearest walkable cell to the screen-centre point that keeps clear of agents and seats
import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnPoint } from '../nav/groundmath.js';

const open = () => true;
const door = { x: 50, z: 50 };

test('open floor with nothing near spawns right on the point, facing the given yaw', () => {
  const r = spawnPoint(open, 3, 4, { avoid: [], fallback: door, yaw: 1.5 });
  assert.deepEqual([r.x, r.z, r.yaw, r.fallback], [3, 4, 1.5, false]);
});

test('a spawn stays at least 0.8 from every agent and seat and faces the point', () => {
  const avoid = [{ x: 0, z: 0 }, { x: 0.5, z: 0.3 }, { x: -1, z: 0.2 }];
  const r = spawnPoint(open, 0, 0, { avoid, fallback: door, yaw: 0 });
  assert.equal(r.fallback, false);
  for (const a of avoid) assert.ok(Math.hypot(a.x - r.x, a.z - r.z) >= 0.8 - 1e-9);
  const want = Math.atan2(0 - r.x, 0 - r.z);
  assert.ok(Math.abs(Math.sin(r.yaw - want)) < 1e-9 && Math.cos(r.yaw - want) > 0);
});

test('an unwalkable point moves to the nearest walkable cell', () => {
  const walk = (x) => x >= 2;
  const r = spawnPoint(walk, 0, 0, { avoid: [], fallback: door, yaw: 0 });
  assert.ok(r.x >= 2 && r.x < 2.5 && Math.abs(r.z) < 0.8);
});

test('nothing clear within 6 units gives the door spawn', () => {
  const r = spawnPoint((x, z) => Math.hypot(x, z) > 7, 0, 0, { avoid: [], fallback: door, yaw: 0.3 });
  assert.deepEqual([r.x, r.z, r.fallback], [50, 50, true]);
  const crowded = spawnPoint(open, 0, 0, { avoid: [{ x: 0, z: 0 }].concat(Array.from({ length: 400 }, (_, i) => ({ x: ((i % 20) - 10) * 0.7, z: (Math.floor(i / 20) - 10) * 0.7 }))), fallback: door, yaw: 0 });
  assert.equal(crowded.fallback, true);
});

test('a non finite point gives the door spawn', () => {
  assert.equal(spawnPoint(open, NaN, 0, { avoid: [], fallback: door, yaw: 0 }).fallback, true);
});
