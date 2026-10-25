// camera presets, fit to bounds and the tween
import test from 'node:test';
import assert from 'node:assert/strict';
import { PRESET_IDS, presetPose, fitBounds, makeTween, stepTween, project } from '../garden/cameras.js';

const bb = { x0: -20, x1: 20, z0: -12, z1: 12, top: 3.4 };

test('the six presets exist in the documented order', () => {
  assert.deepEqual(PRESET_IDS, ['top', 'plan', 'iso', 'hero', 'eye', 'orbit']);
});

test('fit puts every corner of the bounds inside the screen', () => {
  for (const id of ['top', 'plan', 'iso', 'hero', 'orbit']) {
    const pose = presetPose(id, { bb, aspect: 16 / 9 });
    for (const x of [bb.x0, bb.x1]) for (const y of [-1.2, bb.top]) for (const z of [bb.z0, bb.z1]) {
      const p = project(pose, { x, y, z }, 16 / 9);
      assert.ok(Math.abs(p.x) <= 1.001 && Math.abs(p.y) <= 1.001, `${id} corner ${x},${y},${z} -> ${p.x.toFixed(3)},${p.y.toFixed(3)}`);
    }
  }
});

test('fit is tight: some corner reaches the safe edge', () => {
  const pose = presetPose('top', { bb, aspect: 16 / 9 });
  let m = 0;
  for (const x of [bb.x0, bb.x1]) for (const y of [-1.2, bb.top]) for (const z of [bb.z0, bb.z1]) {
    const p = project(pose, { x, y, z }, 16 / 9);
    m = Math.max(m, Math.abs(p.x), Math.abs(p.y));
  }
  assert.ok(m > 0.9 && m <= 0.98, String(m));
});

test('a narrower screen needs a longer distance', () => {
  const wide = fitBounds(bb, { az: 0, el: 75, fov: 30, aspect: 2 });
  const tall = fitBounds(bb, { az: 0, el: 75, fov: 30, aspect: 0.7 });
  assert.ok(tall.dist > wide.dist);
});

test('safe margins shift the target and grow the distance', () => {
  const plain = fitBounds(bb, { az: 0, el: 75, fov: 30, aspect: 16 / 9 });
  const safe = fitBounds(bb, { az: 0, el: 75, fov: 30, aspect: 16 / 9, safe: { left: 0.3, right: 0, top: 0, bottom: 0 } });
  assert.ok(safe.dist > plain.dist);
  assert.ok(safe.target.x < plain.target.x);
});

test('eye uses the layout eye point, with a default when it is missing', () => {
  const eye = { from: [5, 2.5], look: [-8, -0.5] };
  const pose = presetPose('eye', { bb, aspect: 1.6, eye });
  assert.ok(pose.pos && Math.abs(pose.pos.x - 5) < 1e-9 && Math.abs(pose.pos.z - 2.5) < 1e-9);
  assert.ok(pose.pos.y > 0.5 && pose.pos.y < 1.5);
  assert.ok(pose.fov > 40);
  const none = presetPose('eye', { bb, aspect: 1.6 });
  assert.ok(Number.isFinite(none.pos.x));
});

test('tween starts at the old pose, ends at the new one and is monotone', () => {
  const a = presetPose('top', { bb, aspect: 1.6 });
  const b = presetPose('hero', { bb, aspect: 1.6 });
  const tw = makeTween(a, b, 0.8);
  let cur = stepTween(tw, 0);
  assert.ok(Math.abs(cur.az - a.az) < 1e-9);
  let last = cur.el;
  for (let i = 0; i < 100; i++) {
    cur = stepTween(tw, 0.02);
    assert.ok(cur.el <= last + 1e-9);
    last = cur.el;
  }
  assert.ok(tw.done);
  assert.ok(Math.abs(cur.el - b.el) < 1e-9 && Math.abs(cur.dist - b.dist) < 1e-6);
});

// the fog has to follow the camera distance, or a far preset (iso, fov 10) is washed out
test('fog and far plane grow with the camera distance', async () => {
  const { fogRange } = await import('../garden/cameras.js');
  const near = fogRange(40);
  const far = fogRange(400);
  assert.ok(far.near > near.near && far.far > near.far && far.camFar > near.camFar);
  assert.ok(far.near >= 400 * 1.9 && far.camFar >= far.far);
  assert.deepEqual(fogRange(10), { near: 70, far: 220, camFar: 400 });
});

// how much of the safe rect the projected bounds box covers: the larger of width and height fraction
function fill(id, box, safe, aspect = 16 / 9) {
  const pose = presetPose(id, { bb: box, aspect, safe });
  let x0 = 9; let x1 = -9; let y0 = 9; let y1 = -9;
  for (const x of [box.x0, box.x1]) for (const y of [box.bottom ?? -1.2, box.top ?? 3.4]) for (const z of [box.z0, box.z1]) {
    const p = project(pose, { x, y, z }, aspect);
    x0 = Math.min(x0, p.x); x1 = Math.max(x1, p.x); y0 = Math.min(y0, p.y); y1 = Math.max(y1, p.y);
  }
  const sw = 1 - (safe.left || 0) - (safe.right || 0);
  const sh = 1 - (safe.top || 0) - (safe.bottom || 0);
  return { w: (x1 - x0) / 2 / sw, h: (y1 - y0) / 2 / sh };
}

test('iso and hero fill the safe rect for garden and hall bounds', () => {
  const hall = { x0: -14, x1: 14, z0: -9, z1: 9 };
  const safes = [{}, { left: 0.2, right: 0, top: 0.1, bottom: 0.12 }, { left: 0, right: 0.25, top: 0.08, bottom: 0.15 }];
  for (const box of [bb, hall]) for (const id of ['iso', 'hero']) for (const safe of safes) {
    const f = fill(id, box, safe);
    assert.ok(Math.max(f.w, f.h) >= 0.9, `${id} ${JSON.stringify(safe)} ${f.w.toFixed(2)} ${f.h.toFixed(2)}`);
    assert.ok(Math.max(f.w, f.h) <= 1.0, `${id} overfull ${f.w.toFixed(2)} ${f.h.toFixed(2)}`);
  }
});
