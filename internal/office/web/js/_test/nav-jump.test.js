// jump arc, landing and camera pull-in maths, and the clip choice for the avatar
import test from 'node:test';
import assert from 'node:assert/strict';
import { JUMP_H, canJump, startJump, stepJump } from '../nav/jump.js';
import { pullDist, easeDist, camOffset } from '../nav/camfit.js';
import { clipFor } from '../nav/clipmath.js';

test('a jump peaks near 0.6 and lands back at zero height', () => {
  const j = { h: 0, vy: 0 };
  startJump(j);
  let peak = 0;
  let landed = false;
  for (let i = 0; i < 200 && !landed; i++) {
    landed = stepJump(j, 0.005).landed;
    peak = Math.max(peak, j.h);
  }
  assert.ok(Math.abs(peak - JUMP_H) < 0.02, 'peak ' + peak);
  assert.ok(landed);
  assert.equal(j.h, 0);
  assert.equal(j.vy, 0);
});

test('only a grounded walker off the stairs can jump', () => {
  assert.equal(canJump({ h: 0, vy: 0 }, false), true);
  assert.equal(canJump({ h: 0, vy: 0 }, true), false);
  assert.equal(canJump({ h: 0.3, vy: 1 }, false), false);
  const j = { h: 0, vy: 0 };
  assert.equal(startJump(j, true), false);
  assert.equal(j.vy, 0);
});

test('a ceiling (a deck overhead) caps the height and stops the rise', () => {
  const j = { h: 0, vy: 0 };
  startJump(j);
  let max = 0;
  for (let i = 0; i < 200; i++) { stepJump(j, 0.005, 0.3); max = Math.max(max, j.h); }
  assert.ok(max <= 0.3 + 1e-9, 'max ' + max);
});

test('pull-in keeps the camera in front of a hit and never past the wanted distance', () => {
  assert.equal(pullDist(3.2, null), 3.2);
  assert.equal(pullDist(3.2, 5), 3.2);
  assert.ok(Math.abs(pullDist(3.2, 2, 0.2) - 1.8) < 1e-9);
  assert.ok(pullDist(3.2, 0.1, 0.2) >= 0.3, 'floor at the minimum distance');
});

test('the camera snaps in fast and eases back out slowly, or jumps when reduced', () => {
  const inn = easeDist(3, 1, 0.016, false);
  const out = easeDist(1, 3, 0.016, false);
  assert.ok(3 - inn > out - 1, 'in is faster than out');
  assert.ok(inn > 1 && out < 3);
  assert.equal(easeDist(1, 3, 0.016, true), 3);
});

test('a pulled-in camera sits higher and closer, a free one stays at the default', () => {
  const free = camOffset(3.2, 3.2);
  const tight = camOffset(1.6, 3.2);
  assert.equal(free.back, 3.2);
  assert.ok(tight.back < free.back && tight.up > free.up);
});

test('clip follows motion: idle, walk, sprint, jump, fall, with speed driving the rate', () => {
  assert.deepEqual(clipFor({ speed: 0, running: false, air: false, vy: 0 }), { name: 'idle', timeScale: 1 });
  const w = clipFor({ speed: 2.2, running: false, air: false, vy: 0 });
  const slow = clipFor({ speed: 1.1, running: false, air: false, vy: 0 });
  assert.equal(w.name, 'walk');
  assert.ok(w.timeScale > slow.timeScale);
  assert.equal(clipFor({ speed: 4.5, running: true, air: false, vy: 0 }).name, 'sprint');
  assert.equal(clipFor({ speed: 2, running: false, air: true, vy: 2 }).name, 'jump');
  assert.equal(clipFor({ speed: 2, running: false, air: true, vy: -2 }).name, 'fall');
  assert.equal(clipFor({ speed: 0.05, running: false, air: false, vy: 0 }).name, 'idle');
});
