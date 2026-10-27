// the pure bounds math of the sky camera: target clamp, fitted max distance, camera kept inside the walls
import test from 'node:test';
import assert from 'node:assert/strict';
import { clampTarget, maxDistFor, camDistCap, boundPose } from '../nav/orbitbounds.js';
import { dirOf } from '../nav/skymath.js';

const RAD = Math.PI / 180;
const b = { minX: -10, maxX: 10, minZ: -6, maxZ: 6 };
const near = (a, c, eps = 1e-6) => assert.ok(Math.abs(a - c) < eps, `${a} vs ${c}`);

test('target outside the bounds goes to the edge, inside stays', () => {
  assert.deepEqual(clampTarget({ x: 30, y: 0.5, z: -30 }, b), { x: 10, y: 0.5, z: -6 });
  assert.deepEqual(clampTarget({ x: 2, y: 0, z: 3 }, b), { x: 2, y: 0, z: 3 });
});

test('soft clamp moves part of the way and never overshoots', () => {
  const t = clampTarget({ x: 20, y: 0, z: 0 }, b, 0.5);
  near(t.x, 15);
  const s = clampTarget({ x: 10.0004, y: 0, z: 0 }, b, 0.5);
  assert.equal(s.x, 10);
});

test('no bounds leaves the target alone', () => {
  assert.deepEqual(clampTarget({ x: 99, y: 0, z: 99 }, null), { x: 99, y: 0, z: 99 });
});

test('max distance fits the area: bigger area, wider aspect and steeper tilt change it sensibly', () => {
  const base = { bounds: b, az: 0, el: 80 * RAD, fov: 30, aspect: 1.6 };
  const d = maxDistFor(base);
  assert.ok(d > 10 && d < 200, String(d));
  assert.ok(maxDistFor({ ...base, bounds: { minX: -20, maxX: 20, minZ: -12, maxZ: 12 } }) > d * 1.9);
  // a narrow screen needs more distance to hold the width
  assert.ok(maxDistFor({ ...base, aspect: 0.6 }) > d);
  // at the max the footprint width is inside the view (hw / (dist * tan(fov/2) * aspect) <= 1 with the slack)
  const tv = Math.tan(15 * RAD);
  assert.ok(10 / (d * tv * 1.6) <= 1);
});

test('max distance follows the heading: looking along the long side needs the width of the short one', () => {
  const o = { bounds: b, el: 60 * RAD, fov: 30, aspect: 1.6 };
  assert.ok(maxDistFor({ ...o, az: 0 }) !== maxDistFor({ ...o, az: Math.PI / 2 }));
});

test('high tilt leaves the cap at the fitted distance', () => {
  const p = { az: 0.3, el: 80 * RAD, target: { x: 0, y: 0, z: 0 } };
  assert.equal(camDistCap({ ...p, bounds: b, maxDist: 40 }), 40);
});

test('low tilt keeps the camera inside the walls', () => {
  const p = { az: 0, el: 10 * RAD, target: { x: 0, y: 0, z: 0 } };
  const cap = camDistCap({ ...p, bounds: b, maxDist: 60 });
  const d = dirOf(p.az, p.el);
  // az 0 looks along -z from +z: the camera stays within z <= 6 minus the inset
  assert.ok(d.z * cap <= 6 + 1e-9, String(d.z * cap));
  assert.ok(cap < 10);
});

test('the cap grows continuously with tilt', () => {
  let prev = 0;
  for (let deg = 5; deg <= 85; deg += 1) {
    const cap = camDistCap({ az: 0.4, el: deg * RAD, target: { x: 1, y: 0, z: 1 }, bounds: b, maxDist: 50 });
    assert.ok(Math.abs(cap - prev) < 5 || prev === 0, `jump at ${deg}: ${prev} -> ${cap}`);
    prev = cap;
  }
});

test('boundPose clamps the target and the distance together', () => {
  const q = boundPose({ az: 0, el: 20 * RAD, dist: 90, target: { x: 50, y: 0, z: 50 } }, { bounds: b, fov: 30, aspect: 1.6 });
  assert.ok(q.target.x <= 10 && q.target.z <= 6 && q.target.z >= -6);
  assert.ok(q.dist <= 90);
  const cx = q.target.x + dirOf(q.az, q.el).x * q.dist;
  const cz = q.target.z + dirOf(q.az, q.el).z * q.dist;
  assert.ok(cx <= 10.0001 && cz <= 6.0001, `${cx} ${cz}`);
});

test('boundPose is a no-op for a pose already inside, and idempotent', () => {
  const p = { az: 0.5, el: 70 * RAD, dist: 12, target: { x: 1, y: 0, z: 1 } };
  const o = { bounds: b, fov: 30, aspect: 1.6 };
  const q = boundPose(p, o);
  assert.deepEqual(q, p);
  const far = boundPose({ ...p, dist: 500 }, o);
  assert.deepEqual(boundPose(far, o), far);
});

test('boundPose never drops below the minimum distance', () => {
  const q = boundPose({ az: 0, el: 5 * RAD, dist: 30, target: { x: 0, y: 0, z: 6 } }, { bounds: b, fov: 30, aspect: 1.6 });
  assert.ok(q.dist >= 2);
});

test('a target below the ground is lifted to it, with or without bounds', () => {
  assert.equal(clampTarget({ x: 0, y: -4.2, z: 0 }, b).y, 0);
  assert.equal(clampTarget({ x: 0, y: -4.2, z: 0 }, null).y, 0);
  assert.equal(clampTarget({ x: 0, y: -4.2, z: 0 }, b, 1, 0.5).y, 0.5);
  assert.equal(clampTarget({ x: 0, y: 0.6, z: 0 }, b).y, 0.6);
  const p = { az: 0, el: 60 * RAD, dist: 20, target: { x: 0, y: -4.2, z: 0 } };
  assert.equal(boundPose(p, { bounds: b, fov: 30, aspect: 1.6 }).target.y, 0);
  assert.equal(boundPose(p, { bounds: null, fov: 30, aspect: 1.6 }).target.y, 0);
});

test('looking straight down keeps a finite distance (no horizontal heading to fit)', () => {
  const p = { target: { x: 0, y: 0, z: 0 }, az: 0, el: Math.PI / 2 };
  assert.equal(camDistCap({ ...p, bounds: b, maxDist: 40 }), 40);
  const q = boundPose({ ...p, dist: 30 }, { bounds: b, fov: 30, aspect: 1.6 });
  assert.ok(Number.isFinite(q.dist));
});
