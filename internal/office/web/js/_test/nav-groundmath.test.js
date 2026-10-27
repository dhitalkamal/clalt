import test from 'node:test';
import assert from 'node:assert/strict';
import { forwardVec, rightVec, clampPitch, PITCH_MIN, PITCH_MAX, stairFrame, stairCoord, stairHeight, pickSpawn, moveIntent } from '../nav/groundmath.js';

const near = (a, b, e = 1e-9) => assert.ok(Math.abs(a - b) < e, a + ' vs ' + b);

test('forward and right from yaw', () => {
  const f = forwardVec(0);
  near(f[0], 0); near(f[1], 1);
  const f2 = forwardVec(Math.PI / 2);
  near(f2[0], 1); near(f2[1], 0);
  const r = rightVec(0);
  near(r[0], -1); near(r[1], 0);
  // right is a quarter turn clockwise from above
  const r2 = rightVec(Math.PI / 2);
  near(r2[0], 0); near(r2[1], 1);
});

test('pitch clamps to -60..+45 degrees', () => {
  near(clampPitch(10), PITCH_MAX);
  near(clampPitch(-10), PITCH_MIN);
  near(PITCH_MAX, (45 * Math.PI) / 180);
  near(PITCH_MIN, (-60 * Math.PI) / 180);
  near(clampPitch(0.1), 0.1);
});

test('move intent from keys: speed, run, diagonal not faster', () => {
  const w = moveIntent(new Set(['KeyW']), 0);
  near(w.vx, 0); near(w.vz, 2.2);
  const run = moveIntent(new Set(['KeyW', 'ShiftLeft']), 0);
  near(run.vz, 4.5);
  const d = moveIntent(new Set(['KeyW', 'KeyD']), 0);
  near(Math.hypot(d.vx, d.vz), 2.2);
  assert.ok(d.vx > 0 === false, 'D strafes right = -x at yaw 0');
  const s = moveIntent(new Set(['ArrowDown']), 0);
  near(s.vz, -2.2);
  const none = moveIntent(new Set(), 1);
  assert.equal(none.moving, false);
  const both = moveIntent(new Set(['KeyW', 'KeyS']), 0);
  assert.equal(both.moving, false);
});

test('stair ramp height and coordinates', () => {
  const st = { B: { x: 4, z: 2 }, T: { x: 4, z: -1.5 }, dir: [0, -1], rise: 1.29 };
  const fr = stairFrame(st);
  near(fr.len, 3.5);
  const c = stairCoord(fr, 4.2, 0.5);
  near(c.s, 1.5); near(c.lat, 0.2);
  near(stairHeight(fr, 0), 0);
  near(stairHeight(fr, 3.5), 1.29);
  near(stairHeight(fr, 1.75), 0.645);
  near(stairHeight(fr, -2), 0);
  near(stairHeight(fr, 9), 1.29);
});

test('spawn picks the point, else the nearest walkable, else fallback', () => {
  const ok = (x, z) => x >= 0 && x <= 4 && z >= 0 && z <= 4;
  assert.deepEqual(pickSpawn(ok, 1, 1, { x: 9, z: 9 }), { x: 1, z: 1 });
  const p = pickSpawn(ok, 6, 2, { x: 9, z: 9 });
  assert.ok(ok(p.x, p.z) && p.x > 3.5, JSON.stringify(p));
  assert.deepEqual(pickSpawn(() => false, 6, 2, { x: 9, z: 9 }), { x: 9, z: 9 });
  assert.deepEqual(pickSpawn(ok, NaN, 2, { x: 9, z: 9 }), { x: 9, z: 9 });
});
