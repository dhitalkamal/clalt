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
