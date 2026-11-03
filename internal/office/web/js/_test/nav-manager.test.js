// mode manager state machine with fake controllers
import test from 'node:test';
import assert from 'node:assert/strict';
import { createNav } from '../nav/index.js';

const cam = () => ({
  position: { x: 0, y: 10, z: 10, set(x, y, z) { this.x = x; this.y = y; this.z = z; } },
  fov: 30, updateProjectionMatrix() {}, updateMatrixWorld() {},
  lookAt(x, y, z) { this.look = { x, y, z }; },
});
const mkctx = (reduced = false) => ({ stage: { camera: cam(), controls: { target: { x: 0, y: 0, z: 0 } } }, app: {}, garden: {}, reducedMotion: reduced });
const fake = (name, log, extra = {}) => () => ({
  name,
  enter() { log.push('enter ' + name); },
  update() { log.push('update ' + name); return extra.ret; },
  exit() { log.push('exit ' + name); return { pos: [0, 5, 5], target: [0, 0, 0] }; },
  dispose() { log.push('dispose ' + name); },
  ...extra.ctrl,
});

test('starts in sky and switches to ground and back', async () => {
  const log = [];
  const nav = createNav(mkctx(true), { factories: { sky: fake('sky', log), ground: fake('ground', log) } });
  await nav.ready;
  assert.equal(nav.mode, 'sky');
  await nav.setMode('ground');
  assert.equal(nav.mode, 'ground');
  assert.deepEqual(log.filter((l) => !l.startsWith('update')), ['enter sky', 'exit sky', 'dispose sky', 'enter ground']);
  await nav.setMode('sky');
  assert.equal(nav.mode, 'sky');
});

test('a tween runs for 0.6 s and then the controller drives alone', async () => {
  const log = [];
  const nav = createNav(mkctx(false), { factories: { sky: fake('sky', log), ground: fake('ground', log) } });
  await nav.ready;
  await nav.setMode('ground');
  assert.equal(nav.tweening, true);
  nav.update(0.3);
  assert.equal(nav.tweening, true);
  nav.update(0.31);
  assert.equal(nav.tweening, false);
});

test('follow ending returns to the previous mode', async () => {
  const log = [];
  let ended = false;
  const nav = createNav(mkctx(true), { factories: { sky: fake('sky', log), ground: fake('ground', log), follow: () => ({ ...fake('follow', log)(), update() { return ended ? 'back' : undefined; } }) } });
  await nav.ready;
  await nav.setMode('ground');
  await nav.follow('a1');
  assert.equal(nav.mode, 'follow');
  ended = true;
  nav.update(0.016);
  await new Promise((r) => setTimeout(r, 0));
  assert.equal(nav.mode, 'ground');
});

test('a missing ground module falls back to sky', async () => {
  const log = [];
  const nav = createNav(mkctx(true), { factories: { sky: fake('sky', log), ground: null, follow: null } });
  await nav.ready;
  assert.equal(await nav.setMode('ground'), false);
  assert.equal(nav.mode, 'sky');
  assert.equal(await nav.follow('x'), false);
  assert.equal(nav.mode, 'sky');
});

// the fake ground controller keeps the view it was entered with
const groundView = (log) => (o) => ({ ...fake('ground', log)(), enter() { log.push('view ' + (o && o.view)); }, setView(v) { log.push('setView ' + v); } });
const tick = () => new Promise((r) => setTimeout(r, 0));

test('V outside ground only flips the view and keeps the mode', async () => {
  const log = [];
  const nav = createNav(mkctx(true), { factories: { sky: fake('sky', log), ground: groundView(log) } });
  await nav.ready;
  await nav.toggleView();
  assert.equal(nav.mode, 'sky');
  assert.equal(nav.view, 'first');
  await nav.toggleGround();
  assert.ok(log.includes('view first'), log.join());
});

test('entering ground uses the current view', async () => {
  const log = [];
  const nav = createNav(mkctx(true), { factories: { sky: fake('sky', log), ground: groundView(log) } });
  await nav.ready;
  await nav.toggleView();
  await nav.toggleGround();
  await nav.toggleGround();
  await nav.toggleGround();
  assert.equal(log.filter((l) => l.startsWith('view ')).pop(), 'view first');
});

test('two quick G presses cancel out', async () => {
  const log = [];
  const slow = async (o) => { await tick(); return groundView(log)(o); };
  const nav = createNav(mkctx(true), { factories: { sky: fake('sky', log), ground: slow } });
  const a = nav.toggleGround();
  const b = nav.toggleGround();
  await Promise.all([a, b]);
  assert.equal(nav.mode, 'sky');
});

test('dispose while a switch is pending leaves no ghost', async () => {
  const log = [];
  const changes = [];
  let release;
  const gate = new Promise((r) => { release = r; });
  const ctx = mkctx(true);
  ctx.onChange = (s) => changes.push(s.mode);
  const nav = createNav(ctx, { factories: { sky: fake('sky', log), ground: async () => { await gate; return groundView(log)(); } } });
  await nav.ready;
  const p = nav.setMode('ground');
  await tick();
  nav.dispose();
  release();
  assert.equal(await p, false);
  assert.equal(nav.mode, 'sky');
  assert.ok(!log.includes('view undefined') && !log.some((l) => l.startsWith('view ')), log.join());
  assert.ok(log.includes('dispose ground'), log.join());
  assert.deepEqual(changes, []);
});

test('follow with an agent that is gone returns false', async () => {
  const log = [];
  const ctx = mkctx(true);
  ctx.app = { agents: new Map() };
  const nav = createNav(ctx, { factories: { sky: fake('sky', log), follow: fake('follow', log) } });
  await nav.ready;
  assert.equal(await nav.follow('ghost'), false);
  assert.equal(nav.mode, 'sky');
});

test('ready rejects when sky is missing', async () => {
  const nav = createNav(mkctx(true), { factories: { sky: async () => null } });
  await assert.rejects(nav.ready);
});

test('refit re-applies the preset through the sky controller', async () => {
  const log = [];
  const calls = [];
  const ctx = mkctx(true);
  ctx.presetPose = (id) => ({ id });
  const nav = createNav(ctx, { factories: { sky: fake('sky', log, { ctrl: { flyToPreset() {}, refit(c, to) { calls.push(to.id); return true; } } }) } });
  await nav.ready;
  assert.equal(nav.refit(), false, 'no preset yet');
  await nav.toPreset('top');
  assert.equal(nav.refit(), true);
  assert.deepEqual(calls, ['top']);
  nav.pause(true);
  assert.equal(nav.refit(), false, 'paused');
  nav.pause(false);
  nav.dispose();
  assert.equal(nav.refit(), false, 'disposed');
});

test('refit does not throw when the controller has none', async () => {
  const log = [];
  const ctx = mkctx(true);
  ctx.presetPose = () => ({});
  const nav = createNav(ctx, { factories: { sky: fake('sky', log, { ctrl: { flyToPreset() {} } }) } });
  await nav.ready;
  await nav.toPreset('top');
  assert.equal(nav.refit(), false);
});
