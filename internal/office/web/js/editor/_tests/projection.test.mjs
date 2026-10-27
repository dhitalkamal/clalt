import test from 'node:test';
import assert from 'node:assert/strict';
import { projectPoint, groundPoint } from '../projection.js';

// camera 10 units above the origin looking straight down, f = 1, aspect 1
const near = 0.1;
const far = 100;
const A = -(far + near) / (far - near);
const B = (-2 * far * near) / (far - near);
const camera = {
  matrixWorld: { elements: [1, 0, 0, 0, 0, 0, -1, 0, 0, 1, 0, 0, 0, 10, 0, 1] },
  matrixWorldInverse: { elements: [1, 0, 0, 0, 0, 0, 1, 0, 0, -1, 0, 0, 0, 0, -10, 1] },
  projectionMatrix: { elements: [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, A, -1, 0, 0, B, 0] },
  projectionMatrixInverse: { elements: [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1 / B, 0, 0, -1, A / B] },
};

test('the point under the camera lands at the screen center', () => {
  const p = projectPoint(camera, { x: 0, y: 0, z: 0 }, 800, 600);
  assert.ok(Math.abs(p.x - 400) < 1e-6 && Math.abs(p.y - 300) < 1e-6);
  assert.equal(p.visible, true);
});

test('groundPoint inverts projectPoint on a plane', () => {
  const w = { x: 2, y: 0, z: -3 };
  const p = projectPoint(camera, w, 800, 600);
  const back = groundPoint(camera, p.x, p.y, 800, 600, 0);
  assert.ok(Math.abs(back.x - 2) < 1e-6 && Math.abs(back.z + 3) < 1e-6);
});

test('groundPoint works for a raised plane and misses a plane behind the camera', () => {
  const w = { x: 1, y: 4, z: 1 };
  const p = projectPoint(camera, w, 800, 600);
  const back = groundPoint(camera, p.x, p.y, 800, 600, 4);
  assert.ok(Math.abs(back.x - 1) < 1e-6 && Math.abs(back.z - 1) < 1e-6);
  assert.equal(groundPoint(camera, 400, 300, 800, 600, 50), null);
});
