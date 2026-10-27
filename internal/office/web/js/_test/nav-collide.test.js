// sliding collision on a fake walkable function
import test from 'node:test';
import assert from 'node:assert/strict';
import { moveSlide, circleFree } from '../nav/collide.js';

// open room x 0..10, z 0..10 with a wall block at x 5..6, z 0..6
const room = (x, z) => x >= 0 && x <= 10 && z >= 0 && z <= 10 && !(x >= 5 && x <= 6 && z <= 6);

test('free move goes through', () => {
  const p = moveSlide({ x: 2, z: 8 }, 1, -0.5, room);
  assert.ok(Math.abs(p.x - 3) < 1e-9 && Math.abs(p.z - 7.5) < 1e-9);
});

test('slides along a wall instead of stopping', () => {
  // walking diagonally into the wall face at x 5: z keeps advancing
  const p = moveSlide({ x: 4.6, z: 3 }, 0.5, 0.5, room);
  assert.ok(p.x <= 5 - 0.25 + 1e-9, 'stays out of the wall: ' + p.x);
  assert.ok(p.z > 3.4, 'z slid: ' + p.z);
});

test('radius keeps the centre away from the wall', () => {
  let p = { x: 2, z: 3 };
  for (let i = 0; i < 100; i++) p = moveSlide(p, 0.1, 0, room);
  assert.ok(p.x <= 4.75 + 1e-9 && p.x > 4.6, 'x ' + p.x);
  assert.ok(circleFree(p.x, p.z, 0.25, room));
});

test('no tunnelling through a thin wall at run speed and 60 ms frames', () => {
  const thin = (x, z) => !(x >= 5 && x <= 5.1);
  let p = { x: 3, z: 3 };
  const step = 4.5 * 0.06;
  for (let i = 0; i < 40; i++) p = moveSlide(p, step, 0, thin);
  assert.ok(p.x < 5, 'x ' + p.x);
  // a big hitch frame must not tunnel either
  p = moveSlide({ x: 4, z: 3 }, 3, 0, thin);
  assert.ok(p.x < 5, 'hitch x ' + p.x);
});

test('inner corner stops without jitter', () => {
  const corner = (x, z) => x <= 5 && z <= 5;
  let p = { x: 3, z: 3 };
  for (let i = 0; i < 50; i++) p = moveSlide(p, 0.2, 0.2, corner);
  assert.ok(p.x <= 4.75 + 1e-9 && p.z <= 4.75 + 1e-9);
  const q = moveSlide(p, 0.2, 0.2, corner);
  assert.ok(Math.abs(q.x - p.x) < 1e-6 && Math.abs(q.z - p.z) < 1e-6);
});

test('a start already inside tight space can still move out', () => {
  const lane = (x, z) => z >= 0 && z <= 0.4; // narrower than the radius
  const p = moveSlide({ x: 0, z: 0.2 }, 1, 0, lane);
  assert.ok(p.x > 0.9);
});
