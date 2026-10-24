import test from 'node:test';
import assert from 'node:assert/strict';
import { createFollow } from '../nav/follow.js';

// a fake agent: matrixWorld places the model at (x,y,z) facing yaw
const fakeAgent = (x, y, z, yaw = 0) => {
  const c = Math.cos(yaw), s = Math.sin(yaw);
  const e = new Array(16).fill(0);
  e[0] = c; e[2] = -s; e[5] = 1; e[8] = s; e[10] = c; e[12] = x; e[13] = y; e[14] = z; e[15] = 1;
  return { model: { matrixWorld: { elements: e }, updateWorldMatrix() {} } };
};
const camera = () => ({ position: { x: 0, y: 0, z: 0, set(a, b, c) { this.x = a; this.y = b; this.z = c; } }, lookAt(x, y, z) { this.look = [x, y, z]; } });
const mk = (agents) => ({ stage: { camera: camera() }, app: { agents: new Map(agents) }, garden: { groundY: () => 0 }, reducedMotion: false });
const idle = () => ({ keys: new Set(), mouse: { buttons: 0, dx: 0, dy: 0 }, wheel: 0, dbl: false });

test('starts not done and trails the agent from behind', () => {
  const ctx = mk([['a', fakeAgent(2, 0, 3, 0)]]);
  const f = createFollow('a');
  f.enter(ctx, { pos: [0, 5, 0], target: [0, 0, 0] }, {});
  assert.equal(f.done, false);
  for (let i = 0; i < 200; i++) f.update(ctx, 0.016, idle());
  const p = ctx.stage.camera.position;
  assert.ok(Math.abs(p.x - 2) < 0.05, 'x ' + p.x);
  assert.ok(Math.abs(p.z - (3 - 3.5)) < 0.05, 'z ' + p.z);
  assert.ok(Math.abs(p.y - 2.2) < 0.05, 'y ' + p.y);
  assert.equal(f.done, false);
});

test('done when the agent disappears', () => {
  const ctx = mk([['a', fakeAgent(0, 0, 0)]]);
  const f = createFollow('a');
  f.enter(ctx, { pos: [0, 5, 0], target: [0, 0, 0] }, {});
  ctx.app.agents.delete('a');
  f.update(ctx, 0.016, idle());
  assert.equal(f.done, true);
});

test('done on a movement key or escape', () => {
  for (const code of ['KeyW', 'KeyA', 'KeyS', 'KeyD', 'ArrowUp', 'ArrowLeft', 'ArrowDown', 'ArrowRight', 'Escape']) {
    const ctx = mk([['a', fakeAgent(0, 0, 0)]]);
    const f = createFollow('a');
    f.enter(ctx, { pos: [0, 5, 0], target: [0, 0, 0] }, {});
    f.update(ctx, 0.016, { ...idle(), keys: new Set([code]) });
    assert.equal(f.done, true, code);
  }
  const ctx = mk([['a', fakeAgent(0, 0, 0)]]);
  const f = createFollow('a');
  f.enter(ctx, { pos: [0, 5, 0], target: [0, 0, 0] }, {});
  f.update(ctx, 0.016, { ...idle(), keys: new Set(['KeyV', 'ShiftLeft']) });
  assert.equal(f.done, false);
});

test('exit returns the current camera pose', () => {
  const ctx = mk([['a', fakeAgent(0, 0, 0)]]);
  const f = createFollow('a');
  f.enter(ctx, { pos: [0, 5, 0], target: [0, 0, 0] }, {});
  f.update(ctx, 0.016, idle());
  const pose = f.exit(ctx);
  assert.equal(pose.pos.length, 3);
  assert.equal(pose.target.length, 3);
});
