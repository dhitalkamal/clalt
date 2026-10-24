// the ground controller on a real fixture with a fake ctx and an injected avatar
import test from 'node:test';
import assert from 'node:assert/strict';
import { fixture } from './garden-fixtures.js';
import { gardenWalk } from '../garden/walk.js';
import { stairsOf } from '../garden/walkgrid.js';
import { createGround } from '../nav/ground.js';

const cam = () => ({ position: { x: 0, y: 0, z: 0, set(a, b, c) { this.x = a; this.y = b; this.z = c; } }, lookAt(x, y, z) { this.look = [x, y, z]; } });
const setup = (name = 'village-3') => {
  const walk = gardenWalk(fixture('gen-' + name));
  const ctx = { stage: { camera: cam(), scene: {} }, app: { agents: new Map() }, garden: { walk, bounds: {}, groundY: () => 0 }, reducedMotion: false };
  const log = { visible: [], disposed: 0 };
  const makeAvatar = () => ({ update() {}, setVisible(v) { log.visible.push(v); }, dispose() { log.disposed++; } });
  return { walk, ctx, log, makeAvatar };
};
const keys = (...k) => ({ keys: new Set(k), mouse: { buttons: 0, dx: 0, dy: 0 }, wheel: 0, dbl: false });
const run = (g, ctx, input, secs) => { for (let i = 0; i < Math.round(secs / 0.016); i++) g.update(ctx, 0.016, input); };

test('spawns under the screen centre when walkable, else nearest, else gate', () => {
  const { walk, ctx, makeAvatar } = setup();
  const g = createGround({ makeAvatar });
  const sp = walk.spawn;
  g.enter(ctx, { pos: [sp.x, 8, sp.z - 4], target: [sp.x, 0, sp.z] }, {});
  assert.ok(walk.walkable(g.state.x, g.state.z, 0));
  assert.ok(Math.hypot(g.state.x - sp.x, g.state.z - sp.z) < 1.5);
  const g2 = createGround({ makeAvatar });
  g2.enter(ctx, { pos: [500, 8, 500], target: [500, 0, 500] }, {});
  assert.ok(Math.hypot(g2.state.x - sp.x, g2.state.z - sp.z) < 40 && walk.walkable(g2.state.x, g2.state.z, 0));
});

test('W walks forward, shift runs faster, never ends in a blocked cell', () => {
  const { walk, ctx, makeAvatar } = setup();
  const g = createGround({ makeAvatar });
  const sp = walk.spawn;
  g.enter(ctx, { pos: [sp.x - 3, 8, sp.z], target: [sp.x, 0, sp.z] }, {});
  const x0 = g.state.x;
  run(g, ctx, keys('KeyW'), 1);
  const walked = Math.abs(g.state.x - x0) + Math.abs(g.state.z - sp.z);
  assert.ok(walked > 1.5, 'walked ' + walked);
  for (let i = 0; i < 400; i++) { g.update(ctx, 0.06, keys('KeyW', 'ShiftLeft', i % 40 < 20 ? 'KeyQ' : 'KeyE')); assert.ok(walk.walkable(g.state.x, g.state.z, g.state.level)); }
});

test('mouse drag turns and clamps pitch, Q turns', () => {
  const { ctx, makeAvatar } = setup();
  const g = createGround({ makeAvatar });
  g.enter(ctx, { pos: [0, 8, -3], target: [0, 0, 0] }, {});
  const y0 = g.state.yaw;
  g.update(ctx, 0.016, { keys: new Set(), mouse: { buttons: 1, dx: 50, dy: 5000 }, wheel: 0 });
  assert.ok(g.state.yaw < y0);
  assert.ok(Math.abs(g.state.pitch + (60 * Math.PI) / 180) < 1e-9);
  const y1 = g.state.yaw;
  g.update(ctx, 0.1, keys('KeyQ'));
  assert.ok(g.state.yaw > y1);
});

test('climbs the treehouse stairs and walks back down', () => {
  const { walk, ctx, makeAvatar } = setup();
  const st = stairsOf(walk.site);
  const g = createGround({ makeAvatar });
  const [dx, dz] = st.dir;
  const sx = st.B.x - dx * 0.8;
  const sz = st.B.z - dz * 0.8;
  g.enter(ctx, { pos: [sx - dx * 3, 5, sz - dz * 3], target: [sx, 0, sz] }, {});
  assert.equal(g.state.level, 0);
  let seenRamp = false;
  for (let i = 0; i < 500 && g.state.level === 0; i++) {
    g.update(ctx, 0.016, keys('KeyW'));
    if (g.state.y > 0.2 && g.state.y < 1.2) seenRamp = true;
  }
  assert.equal(g.state.level, 1, 'reached the deck');
  assert.ok(seenRamp, 'y followed the ramp');
  assert.ok(Math.abs(g.state.y - st.rise) < 1e-6);
  // turn around and go back down
  run(g, ctx, keys('KeyQ'), 1.75);
  let minY = 9;
  for (let i = 0; i < 800 && g.state.level === 1; i++) { g.update(ctx, 0.016, keys('KeyW')); minY = Math.min(minY, g.state.y); }
  assert.equal(g.state.level, 0, 'back on the ground');
  assert.ok(g.state.y < 0.3);
});

test('first person sits at eye height, setView swaps, exit returns a pose, dispose frees the avatar', async () => {
  const { ctx, log, makeAvatar } = setup();
  const g = createGround({ view: 'first', makeAvatar });
  g.enter(ctx, { pos: [0, 8, -3], target: [0, 0, 0] }, {});
  await new Promise((r) => setTimeout(r, 5));
  g.update(ctx, 0.016, keys());
  assert.ok(Math.abs(ctx.stage.camera.position.y - 0.9) < 1e-6);
  assert.equal(log.visible.at(-1), false);
  g.setView('third');
  g.update(ctx, 0.016, keys());
  assert.equal(log.visible.at(-1), true);
  const pose = g.exit(ctx);
  assert.equal(pose.pos.length, 3);
  assert.equal(pose.target.length, 3);
  g.dispose();
  assert.equal(log.disposed, 1);
});
