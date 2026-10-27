// the ground walker on the hall: up the stairs to the deck and back down
import test from 'node:test';
import assert from 'node:assert/strict';
import { hallCompiled } from './hall-fixture.js';
import { hallCtx } from '../nav/hallctx.js';
import { createGround } from '../nav/ground.js';

const cam = () => ({ position: { x: 0, y: 0, z: 0, set(a, b, c) { this.x = a; this.y = b; this.z = c; } }, lookAt() {} });
const keys = (...k) => ({ keys: new Set(k), mouse: { buttons: 0, dx: 0, dy: 0 }, wheel: 0, dbl: false });
const run = (g, ctx, input, secs) => { for (let i = 0; i < Math.round(secs / 0.016); i++) g.update(ctx, 0.016, input); };

function setup() {
  const L = hallCompiled();
  const h = hallCtx(L);
  const ctx = { stage: { camera: cam(), scene: {} }, app: { agents: new Map() }, garden: { walk: h.walk, bounds: h.bounds, groundY: h.groundY }, reducedMotion: false };
  const makeAvatar = () => ({ update() {}, setVisible() {}, dispose() {} });
  return { L, h, ctx, makeAvatar };
}

test('spawns at the door on level 0 when the view points nowhere useful', () => {
  const { h, ctx, makeAvatar } = setup();
  const g = createGround({ makeAvatar });
  g.enter(ctx, { pos: [900, 8, 900], target: [900, 0, 900] }, {});
  assert.equal(g.state.level, 0);
  assert.ok(h.walk.walkable(g.state.x, g.state.z, 0));
});

test('climbs the hall stairs to the deck and walks back down', () => {
  const { h, ctx, makeAvatar } = setup();
  const s = h.stairs;
  const g = createGround({ makeAvatar });
  const [dx, dz] = s.dir;
  const sx = s.foot.x - dx * 0.8;
  const sz = s.foot.z - dz * 0.8;
  g.enter(ctx, { pos: [sx - dx * 3, 5, sz - dz * 3], target: [sx, 0, sz] }, {});
  assert.equal(g.state.level, 0);
  let ramp = false;
  for (let i = 0; i < 600 && g.state.level === 0; i++) {
    g.update(ctx, 0.016, keys('KeyW'));
    if (g.state.y > 0.2 && g.state.y < 1.1) ramp = true;
  }
  assert.equal(g.state.level, 1, 'reached the deck');
  assert.ok(ramp, 'y followed the ramp');
  assert.ok(Math.abs(g.state.y - s.height) < 1e-6);
  assert.ok(h.walk.walkable(g.state.x, g.state.z, 1));
  run(g, ctx, keys('KeyQ'), 1.75);
  for (let i = 0; i < 900 && g.state.level === 1; i++) g.update(ctx, 0.016, keys('KeyW'));
  assert.equal(g.state.level, 0, 'back on the floor');
  assert.ok(g.state.y < 0.3);
});

test('walking into furniture or a wall never ends in a blocked cell', () => {
  const { h, ctx, makeAvatar } = setup();
  const g = createGround({ makeAvatar });
  g.enter(ctx, { pos: [-20, 5, 0], target: [-18, 0, 0] }, {});
  for (let i = 0; i < 800; i++) {
    g.update(ctx, 0.05, keys('KeyW', 'ShiftLeft', i % 60 < 30 ? 'KeyQ' : 'KeyE'));
    assert.ok(h.walk.walkable(g.state.x, g.state.z, g.state.level), 'step ' + i);
  }
});
