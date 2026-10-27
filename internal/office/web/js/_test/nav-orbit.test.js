// free orbit math: pivot-fixed rotation, pitch limits, input mapping, key tilt, height and wall clamps
import test from 'node:test';
import assert from 'node:assert/strict';
import { orbitAround, clampOrbit, keyTurn, shortenForWall, stepMomentum, EL_LO, EL_HI } from '../nav/orbit.js';
import { dragMode } from '../nav/input.js';
import { dirOf, rayFromPose, hitGround } from '../nav/skymath.js';

const near = (a, b, eps = 1e-6) => assert.ok(Math.abs(a - b) < eps, `${a} vs ${b}`);
const camOf = (p) => { const d = dirOf(p.az, p.el); return { x: p.target.x + d.x * p.dist, y: p.target.y + d.y * p.dist, z: p.target.z + d.z * p.dist }; };
const base = { az: 0.4, el: 0.9, dist: 30, target: { x: 2, y: 0, z: -3 } };
const pivot = { x: 8, y: 0, z: 5 };

// screen position of a world point for a pose: project onto the camera basis through a ray fan is overkill, so
// compare the pivot in camera-local coordinates, which a rigid rotation about the pivot must keep
function local(p, w) {
  const c = camOf(p);
  const d = dirOf(p.az, p.el);
  const f = { x: -d.x, y: -d.y, z: -d.z };
  const r = { x: Math.cos(p.az), y: 0, z: -Math.sin(p.az) };
  const u = { x: r.y * f.z - r.z * f.y, y: r.z * f.x - r.x * f.z, z: r.x * f.y - r.y * f.x };
  const v = { x: w.x - c.x, y: w.y - c.y, z: w.z - c.z };
  const dot = (a, b) => a.x * b.x + a.y * b.y + a.z * b.z;
  return { r: dot(v, r), u: dot(v, u), f: dot(v, f) };
}

test('orbit keeps the pivot at the same place in the camera frame', () => {
  const q = orbitAround(base, pivot, 1.1, -0.3);
  const a = local(base, pivot); const b = local(q, pivot);
  near(a.r, b.r, 1e-6); near(a.u, b.u, 1e-6); near(a.f, b.f, 1e-6);
  near(q.dist, base.dist);
  near(q.az, 0.4 + 1.1 - 2 * Math.PI * Math.round(1.1 / (2 * Math.PI)));
});

test('orbit around a pivot keeps the camera at the same distance from it', () => {
  const dist0 = Math.hypot(...['x', 'y', 'z'].map((k) => camOf(base)[k] - pivot[k]));
  const q = orbitAround(base, pivot, -2.2, 0.2);
  const dist1 = Math.hypot(...['x', 'y', 'z'].map((k) => camOf(q)[k] - pivot[k]));
  near(dist0, dist1, 1e-6);
});

test('yaw is unlimited and wraps', () => {
  let p = base;
  for (let i = 0; i < 40; i++) p = orbitAround(p, pivot, Math.PI / 5, 0);
  near(p.target.x, base.target.x, 1e-6); near(p.target.z, base.target.z, 1e-6);
  assert.ok(p.az > -Math.PI - 1e-9 && p.az <= Math.PI + 1e-9);
  near(Math.cos(p.az), Math.cos(base.az)); near(Math.sin(p.az), Math.sin(base.az));
});

test('pitch clamps to 3 and 90 degrees', () => {
  assert.equal(orbitAround(base, pivot, 0, -9).el, EL_LO);
  assert.equal(orbitAround(base, pivot, 0, 9).el, EL_HI);
  near(EL_LO, (3 * Math.PI) / 180); near(EL_HI, Math.PI / 2);
});

test('plain left drag orbits, shift and right drag pan', () => {
  assert.equal(dragMode(1, false), 'orbit');
  assert.equal(dragMode(1, true), 'pan');
  assert.equal(dragMode(2, false), 'pan');
  assert.equal(dragMode(2, true), 'pan');
  assert.equal(dragMode(0, false), null);
  assert.equal(dragMode(4, false), null);
});

test('z and x tilt down and up, q and e turn', () => {
  const k = (...c) => new Set(c);
  assert.ok(keyTurn(k('KeyZ'), 0.1).dEl > 0);
  assert.ok(keyTurn(k('KeyX'), 0.1).dEl < 0);
  near(keyTurn(k('KeyX'), 0.1).dEl, -keyTurn(k('KeyZ'), 0.1).dEl);
  assert.equal(keyTurn(k('KeyZ', 'KeyX'), 0.1).dEl, 0);
  assert.ok(keyTurn(k('KeyE'), 0.1).dAz > 0);
  assert.ok(keyTurn(k('KeyQ'), 0.1).dAz < 0);
  assert.deepEqual(keyTurn(k(), 0.1), { dAz: 0, dEl: 0 });
  // tilt down drives the pose toward straight down
  assert.ok(orbitAround(base, base.target, 0, keyTurn(k('KeyZ'), 0.1).dEl).el > base.el);
});

test('camera never goes below 0.6 above the ground when close', () => {
  const p = clampOrbit({ az: 0, el: EL_LO, dist: 4, target: { x: 0, y: 0, z: 0 } }, { groundY: 0 });
  assert.ok(camOf(p).y >= 0.6 - 1e-9);
  assert.equal(p.dist, 4);
  const q = clampOrbit({ az: 0, el: EL_LO, dist: 4, target: { x: 0, y: 1.29, z: 0 } }, { groundY: 1.29 });
  assert.ok(camOf(q).y >= 1.29 + 0.6 - 1e-9);
  // far away the 3 degree tilt is already above the minimum and stays
  assert.equal(clampOrbit({ az: 0, el: EL_LO, dist: 60, target: { x: 0, y: 0, z: 0 } }, { groundY: 0 }).el, EL_LO);
});

test('far limit still caps the camera height and the distance at low tilt', () => {
  const hi = clampOrbit({ az: 0, el: 1.2, dist: 900, target: { x: 0, y: 0, z: 0 } }, { maxH: 120 });
  assert.ok(camOf(hi).y <= 120 + 1e-9);
  const lo = clampOrbit({ az: 0, el: EL_LO, dist: 900, target: { x: 0, y: 0, z: 0 } }, { maxH: 120 });
  assert.ok(lo.dist <= 120 * 1.5 + 1e-9);
});

test('a wall between pivot and camera shortens the distance in front of it', () => {
  near(shortenForWall(30, 10), 9.5);
  assert.equal(shortenForWall(30, null), 30);
  assert.equal(shortenForWall(30, 50), 30);
  assert.ok(shortenForWall(30, 0.2) >= 1);
});

test('momentum decays to zero and is off under reduced motion', () => {
  const v = stepMomentum({ yaw: 2, pitch: 1 }, 0.1, false);
  assert.ok(v.yaw < 2 && v.yaw > 0);
  assert.deepEqual(stepMomentum({ yaw: 2, pitch: 1 }, 0.1, true), { yaw: 0, pitch: 0 });
  assert.deepEqual(stepMomentum({ yaw: 1e-5, pitch: 0 }, 0.1, false), { yaw: 0, pitch: 0 });
});

test('orbit leaves a ground pivot under the same cursor ray', () => {
  // pivot is the ground point under ndc (0.4, -0.2): after orbiting its ray still hits the pivot
  const ndc = { x: 0.4, y: -0.2 };
  const pv = hitGround(rayFromPose(base, ndc.x, ndc.y, 30, 1.6), 0);
  const q = orbitAround(base, pv, 0.9, 0.2);
  const h = hitGround(rayFromPose(q, ndc.x, ndc.y, 30, 1.6), 0);
  near(h.x, pv.x, 1e-5); near(h.z, pv.z, 1e-5);
});

test('picker passes the rays to the injected cast', async () => {
  const { createPicker } = await import('../nav/skypick.js');
  const seen = [];
  const pk = createPicker({}, (o, d, far, floors) => { seen.push(floors); return { distance: 7, point: { x: 1, y: 2, z: 3 } }; });
  assert.deepEqual(pk.pivotAt({ o: {}, d: {} }, 50), { x: 1, y: 2, z: 3 });
  assert.equal(pk.wallAlong({}, {}, 50), 7);
  assert.deepEqual(seen, [true, false]);
  assert.equal(createPicker({}, () => null).wallAlong({}, {}, 5), null);
});
