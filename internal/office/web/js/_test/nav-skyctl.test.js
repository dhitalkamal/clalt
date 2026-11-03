// the sky controller driven with a fake stage: pivot reach, zoom limits, momentum vs presets, refit
import test from 'node:test';
import assert from 'node:assert/strict';
import { createSky } from '../nav/sky.js';

const vec = () => ({ x: 0, y: 0, z: 0, set(x, y, z) { this.x = x; this.y = y; this.z = z; } });
function rig(bounds = { minX: -40, maxX: 40, minZ: -40, maxZ: 40 }) {
  const camera = { position: vec(), up: vec(), fov: 30, lookAt() {}, updateMatrixWorld() {}, updateProjectionMatrix() {} };
  const view = { az: 0, el: 0, dist: 0, target: vec() };
  const ctx = { stage: { camera, view, controls: { target: vec() }, renderer: { domElement: { clientWidth: 800, clientHeight: 600 } } }, garden: { bounds } };
  const sky = createSky();
  sky.enter(ctx, { pos: [0, 40, 40], target: [0, 0, 0] });
  const input = (o = {}) => ({ keys: new Set(), mouse: { x: 400, y: 300, dx: 0, dy: 0, buttons: 0, shift: false, ctrl: false }, wheel: 0, ...o });
  const step = (o, dt = 1 / 60) => sky.update(ctx, dt, input(o));
  return { ctx, sky, step, input, view };
}
const drag = (r, dy, x = 400, y = 590) => {
  for (let i = 0; i < 25; i++) r.step({ mouse: { x, y, dx: 0, dy: dy / 25, buttons: 1, shift: false, ctrl: false } });
  r.step();
};
const preset = { az: 0, el: 1.2, dist: 60, target: { x: 0, y: 0, z: 0 }, fov: 30 };

test('orbit drags from the bottom edge never lift the camera above the cap', () => {
  const r = rig();
  drag(r, -250); drag(r, 350);
  assert.ok(r.view.target.y < 5, `target.y ${r.view.target.y}`);
  assert.ok(r.ctx.stage.camera.position.y <= 121, `cam y ${r.ctx.stage.camera.position.y}`);
});

test('wheel zoom at the max distance leaves the target alone', () => {
  const r = rig();
  for (let i = 0; i < 60; i++) r.step({ wheel: 400 });
  for (let i = 0; i < 200; i++) r.step();
  const t = { ...r.view.target };
  for (let i = 0; i < 30; i++) r.step({ wheel: 400, mouse: { x: 700, y: 100, dx: 0, dy: 0, buttons: 0 } });
  assert.ok(Math.abs(r.view.target.x - t.x) < 1e-6 && Math.abs(r.view.target.z - t.z) < 1e-6, `${t.x} -> ${r.view.target.x}`);
});

test('wheel zoom at the min distance leaves the target alone', () => {
  const r = rig();
  for (let i = 0; i < 80; i++) r.step({ wheel: -400 });
  for (let i = 0; i < 200; i++) r.step();
  const t = { ...r.view.target };
  for (let i = 0; i < 30; i++) r.step({ wheel: -400, mouse: { x: 700, y: 100, dx: 0, dy: 0, buttons: 0 } });
  assert.ok(Math.abs(r.view.target.x - t.x) < 1e-6 && Math.abs(r.view.target.z - t.z) < 1e-6, `${t.x} -> ${r.view.target.x}`);
});

test('release momentum does not cancel a preset fly', () => {
  const r = rig();
  for (let i = 0; i < 10; i++) r.step({ mouse: { x: 400, y: 300, dx: -20, dy: 0, buttons: 1, shift: false, ctrl: false } });
  r.step();
  r.sky.flyToPreset(r.ctx, preset, false);
  for (let i = 0; i < 120; i++) r.step();
  assert.ok(Math.abs(r.view.dist - 60) < 0.5, `dist ${r.view.dist}`);
});

test('release momentum does not cancel a double click fly', () => {
  const r = rig();
  for (let i = 0; i < 10; i++) r.step({ mouse: { x: 400, y: 300, dx: -20, dy: 0, buttons: 1, shift: false, ctrl: false } });
  r.step({ dbl: { x: 400, y: 300 } });
  const d0 = r.view.dist;
  for (let i = 0; i < 90; i++) r.step();
  assert.ok(r.view.dist < d0 * 0.7, `dist ${d0} -> ${r.view.dist}`);
});

test('refit re-applies the preset while the camera is untouched, not after the user moved it', () => {
  const r = rig();
  r.sky.flyToPreset(r.ctx, preset, true);
  const wide = { ...preset, dist: 90 };
  assert.equal(r.sky.refit(r.ctx, wide), true);
  assert.ok(Math.abs(r.view.dist - 90) < 1e-6);
  for (let i = 0; i < 5; i++) r.step({ keys: new Set(['KeyW']) });
  assert.equal(r.sky.refit(r.ctx, preset), false);
  assert.ok(Math.abs(r.view.dist - 90) < 5);
});

test('a wide preset height cap does not outlive the preset', () => {
  const r = rig({ minX: -300, maxX: 300, minZ: -300, maxZ: 300 });
  r.sky.flyToPreset(r.ctx, { az: 0, el: 1.3, dist: 200, target: { x: 0, y: 0, z: 0 }, fov: 30 }, true);
  r.step({ keys: new Set(['KeyW']) });
  r.sky.flyToPreset(r.ctx, { ...preset }, true);
  r.step({ keys: new Set(['KeyW']) });
  for (let i = 0; i < 100; i++) r.step({ wheel: 400 });
  assert.ok(r.ctx.stage.camera.position.y <= 121, `cam y ${r.ctx.stage.camera.position.y}`);
});
