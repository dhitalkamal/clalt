// the ground controller jumping: deck landing, stairs, reduced motion, camera pull-in with a fake ray
import test from 'node:test';
import assert from 'node:assert/strict';
import { hallCompiled } from './hall-fixture.js';
import { hallCtx } from '../nav/hallctx.js';
import { createGround } from '../nav/ground.js';

const cam = () => ({ position: { x: 0, y: 0, z: 0, set(a, b, c) { this.x = a; this.y = b; this.z = c; } }, lookAt() {} });
const keys = (...k) => ({ keys: new Set(k), mouse: { buttons: 0, dx: 0, dy: 0 }, wheel: 0, dbl: false });
const run = (g, ctx, input, secs) => { for (let i = 0; i < Math.round(secs / 0.016); i++) g.update(ctx, 0.016, input); };

function setup(extra = {}, reduced = false) {
  const h = hallCtx(hallCompiled());
  const ctx = { stage: { camera: cam(), scene: {} }, app: { agents: new Map() }, garden: { walk: h.walk, bounds: h.bounds, groundY: h.groundY }, reducedMotion: reduced };
  const seen = [];
  const makeAvatar = () => ({ update(st) { seen.push(st); }, setVisible() {}, dispose() {} });
  const g = createGround({ makeAvatar, ...extra });
  g.enter(ctx, { pos: [900, 8, 900], target: [900, 0, 900] }, {});
  return { h, ctx, g, seen };
}

test('space lifts the walker and it lands on the floor again', () => {
  const { ctx, g } = setup();
  g.update(ctx, 0.016, keys('Space'));
  run(g, ctx, keys('Space'), 0.1);
  assert.ok(g.state.y > 0.1, 'airborne ' + g.state.y);
  let peak = 0;
  for (let i = 0; i < 200; i++) { g.update(ctx, 0.016, keys()); peak = Math.max(peak, g.state.y); }
  assert.ok(peak > 0.5 && peak < 0.7, 'peak ' + peak);
  assert.equal(g.state.y, 0);
});

test('a jump on the deck lands on the deck height', () => {
  const { h, ctx, g } = setup();
  const s = h.stairs;
  const [dx, dz] = s.dir;
  g.enter(ctx, { pos: [s.foot.x - dx * 4, 5, s.foot.z - dz * 4], target: [s.foot.x - dx * 0.8, 0, s.foot.z - dz * 0.8] }, {});
  for (let i = 0; i < 600 && g.state.level === 0; i++) g.update(ctx, 0.016, keys('KeyW'));
  assert.equal(g.state.level, 1);
  const deck = g.state.y;
  g.update(ctx, 0.016, keys('Space'));
  run(g, ctx, keys(), 0.15);
  assert.ok(g.state.y > deck + 0.1);
  run(g, ctx, keys(), 1);
  assert.equal(g.state.y, deck);
  assert.equal(g.state.level, 1);
});

test('space is ignored while on the stairs', () => {
  const { h, ctx, g } = setup();
  const s = h.stairs;
  const [dx, dz] = s.dir;
  g.enter(ctx, { pos: [s.foot.x - dx * 4, 5, s.foot.z - dz * 4], target: [s.foot.x - dx * 0.8, 0, s.foot.z - dz * 0.8] }, {});
  for (let i = 0; i < 600 && !g.state.stairs; i++) g.update(ctx, 0.016, keys('KeyW'));
  assert.ok(g.state.stairs);
  run(g, ctx, keys('KeyW', 'Space'), 0.05);
  assert.ok(g.state.stairs);
  assert.equal(g.state.h, 0);
});

test('reduced motion: the jump works but the camera does not bob', () => {
  const { ctx, g } = setup({}, true);
  run(g, ctx, keys(), 0.1);
  const y0 = ctx.stage.camera.position.y;
  g.update(ctx, 0.016, keys('Space'));
  let dev = 0;
  let peak = 0;
  for (let i = 0; i < 40; i++) { g.update(ctx, 0.016, keys()); peak = Math.max(peak, g.state.h); dev = Math.max(dev, Math.abs(ctx.stage.camera.position.y - y0)); }
  assert.ok(peak > 0.3, 'jumped ' + peak);
  assert.ok(dev < 1e-9, 'camera moved ' + dev);
});

test('a ray hit pulls the third person camera in, a clear ray leaves it at the default', async () => {
  let hit = null;
  const camRay = { hit: () => hit };
  const { ctx, g } = setup({ camRay });
  await new Promise((r) => setTimeout(r, 5));
  const dist = () => Math.hypot(ctx.stage.camera.position.x - g.state.x, ctx.stage.camera.position.z - g.state.z);
  run(g, ctx, keys(), 1);
  const open = dist();
  hit = 1.2;
  run(g, ctx, keys(), 1);
  assert.ok(dist() < open - 0.5, 'pulled in ' + dist() + ' vs ' + open);
  hit = null;
  run(g, ctx, keys(), 3);
  assert.ok(Math.abs(dist() - open) < 0.1, 'eased back');
});

test('a held space jumps once, a new press jumps again', () => {
  const { ctx, g } = setup();
  let jumps = 0;
  let air = false;
  const step = (input) => {
    g.update(ctx, 0.016, input);
    if (g.state.h > 0 && !air) jumps++;
    air = g.state.h > 0;
  };
  for (let i = 0; i < 300; i++) step(keys('Space'));
  assert.equal(jumps, 1, 'held key');
  step(keys());
  for (let i = 0; i < 300; i++) step(keys('Space'));
  assert.equal(jumps, 2, 'second press');
});
