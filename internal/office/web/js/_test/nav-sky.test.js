// sky math and input filtering: pure parts of the navigation code
import test from 'node:test';
import assert from 'node:assert/strict';
import { panVector, clampPose, hitGround, rayFromPose, zoomToCursor, EL_MIN, EL_MAX } from '../nav/skymath.js';
import { ignoreKey } from '../nav/input.js';

const near = (a, b, eps = 1e-6) => assert.ok(Math.abs(a - b) < eps, `${a} vs ${b}`);
const bounds = { minX: -20, maxX: 20, minZ: -12, maxZ: 12 };

test('pan forward goes where the camera looks', () => {
  // az 0: camera south of the target looking north (-z)
  const v = panVector(0, { fwd: 1, right: 0 }, 10);
  near(v.x, 0); near(v.z, -10);
  // az 90 deg: camera east looking west (-x)
  const w = panVector(Math.PI / 2, { fwd: 1, right: 0 }, 10);
  near(w.x, -10); near(w.z, 0);
  const r = panVector(0, { fwd: 0, right: 1 }, 10);
  near(r.x, 10); near(r.z, 0);
});

test('clamp keeps tilt, height and target inside the limits', () => {
  const p = clampPose({ az: 0, el: 0.01, dist: 5, target: { x: 99, y: 0, z: -99 } }, { bounds, margin: 4 });
  assert.equal(p.el, EL_MIN);
  assert.equal(clampPose({ az: 0, el: 3, dist: 5, target: { x: 0, y: 0, z: 0 } }, { bounds }).el, EL_MAX);
  near(p.target.x, 24); near(p.target.z, -16);
  const low = clampPose({ az: 0, el: 1, dist: 0.1, target: { x: 0, y: 0, z: 0 } }, { bounds });
  assert.ok(Math.sin(low.el) * low.dist >= 2 - 1e-9);
  const high = clampPose({ az: 0, el: 1, dist: 900, target: { x: 0, y: 0, z: 0 } }, { bounds });
  assert.ok(Math.sin(high.el) * high.dist <= 120 + 1e-9);
});

test('ray through the screen centre hits the target on the ground', () => {
  const p = { az: 0.7, el: 1, dist: 30, target: { x: 3, y: 0, z: -4 } };
  const h = hitGround(rayFromPose(p, 0, 0, 30, 1.6), 0);
  near(h.x, 3, 1e-6); near(h.z, -4, 1e-6);
});

test('zoom toward the cursor keeps the ground point under the cursor', () => {
  const p = { az: 0.4, el: 0.9, dist: 40, target: { x: 0, y: 0, z: 0 } };
  const ndc = { x: 0.5, y: -0.3 };
  const before = hitGround(rayFromPose(p, ndc.x, ndc.y, 30, 1.6), 0);
  const q = zoomToCursor(p, 0.5, ndc, 30, 1.6);
  assert.ok(q.dist < p.dist);
  const after = hitGround(rayFromPose(q, ndc.x, ndc.y, 30, 1.6), 0);
  near(after.x, before.x, 1e-6); near(after.z, before.z, 1e-6);
});

test('keys are ignored while typing or with ctrl and meta', () => {
  assert.equal(ignoreKey({ target: { tagName: 'INPUT' } }), true);
  assert.equal(ignoreKey({ target: { tagName: 'DIV', isContentEditable: true } }), true);
  assert.equal(ignoreKey({ target: { tagName: 'DIV' }, ctrlKey: true }), true);
  assert.equal(ignoreKey({ target: { tagName: 'DIV' }, metaKey: true }), true);
  assert.equal(ignoreKey({ target: { tagName: 'DIV' } }), false);
});
