// the nav host: one manager per template, rebuilt on a switch between the hall and the garden
import test from 'node:test';
import assert from 'node:assert/strict';
import { createNavHost } from '../nav/host.js';
import { hallCompiled, hallLayout } from './hall-fixture.js';
import { fixture } from './garden-fixtures.js';
import { gardenWalk } from '../garden/walk.js';
import { presetPose, project } from '../garden/cameras.js';

const cam = () => ({
  position: { x: 0, y: 10, z: 10, set(x, y, z) { this.x = x; this.y = y; this.z = z; } },
  fov: 30, updateProjectionMatrix() {}, updateMatrixWorld() {}, lookAt() {},
});

function setup() {
  const log = { made: 0, disposed: 0, ctxs: [], entered: [] };
  const stage = { camera: cam(), controls: { target: { x: 0, y: 0, z: 0 } }, rig: null, nav: null };
  const app = { layout: null, layoutJson: null, agents: new Map() };
  const fake = (name) => () => ({
    name, enter(ctx) { log.entered.push([name, ctx.garden.walk]); }, update() {}, exit() { return { pos: [0, 5, 5], target: [0, 0, 0] }; },
    flyToPreset() {}, dispose() { log.disposed++; },
  });
  const host = createNavHost({
    stage, app,
    load: async () => ({
      createNav: (ctx, o) => { log.made++; log.ctxs.push(ctx); return globalThis.__realCreateNav(ctx, { ...o, factories: { sky: fake('sky'), ground: fake('ground'), follow: fake('follow') }, input: { keys: new Set(), mouse: {}, wheel: 0, consume() {}, dispose() { log.disposed++; } } }); },
      presetPose, fitFog() {},
    }),
    reduced: () => true,
    aspect: () => 16 / 9,
  });
  return { log, stage, app, host };
}

const hallApp = (app) => { app.layout = hallCompiled(); app.layoutJson = hallLayout(); };
const gardenApp = (app, stage) => {
  const walk = gardenWalk(fixture('gen-village-3'));
  app.layout = { v: 2, walk };
  app.layoutJson = { v: 2 };
  stage.rig = { bb: { x0: -20, x1: 20, z0: -12, z1: 12 }, aspect: () => 16 / 9, safe: {}, eye: null };
  return walk;
};

test.before(async () => { globalThis.__realCreateNav = (await import('../nav/index.js')).createNav; });

test('the manager starts on the hall with the hall walk and bounds', async () => {
  const { log, stage, app, host } = setup();
  hallApp(app);
  const nav = await host.get();
  assert.ok(nav);
  assert.equal(stage.nav, nav);
  await nav.setMode('ground');
  assert.equal(log.entered.at(-1)[0], 'ground');
  assert.equal(log.entered.at(-1)[1], app.layout && log.ctxs[0].garden.walk);
  assert.ok(log.ctxs[0].garden.walk.walkable(app.layout.X(10.5), app.layout.Z(8.5), 0) !== undefined);
  assert.equal(log.ctxs[0].garden.bounds.maxX, app.layout.xmin + app.layout.W);
  const pose = log.ctxs[0].presetPose('top');
  assert.ok(pose && pose.dist > 10);
});

test('a hall preset honours the safe margins', async () => {
  const { log, app, host } = setup();
  hallApp(app);
  await host.get();
  const plain = log.ctxs[0].presetPose('top');
  host.setSafe({ left: 0.25, bottom: 0.1, top: 0.02 });
  const safe = log.ctxs[0].presetPose('top');
  assert.ok(safe.dist > plain.dist);
  assert.ok(safe.target.x < plain.target.x);
});

test('switching hall to garden and back disposes the old manager and builds a new one', async () => {
  const { log, stage, app, host } = setup();
  hallApp(app);
  const a = await host.get();
  assert.equal(log.made, 1);
  host.reset();
  assert.equal(stage.nav, null);
  assert.ok(log.disposed >= 1, 'input of the old manager is disposed');
  const walk = gardenApp(app, stage);
  const b = await host.get();
  assert.notEqual(a, b);
  assert.equal(stage.nav, b);
  await b.setMode('ground');
  assert.equal(log.entered.at(-1)[1], walk);
  const d0 = log.disposed;
  host.reset();
  hallApp(app);
  const c = await host.get();
  assert.notEqual(b, c);
  assert.ok(log.disposed > d0);
  assert.equal(log.made, 3);
  await c.setMode('ground');
  assert.equal(log.entered.at(-1)[1], log.ctxs[2].garden.walk);
  assert.ok(log.entered.at(-1)[1].stairDesc);
});

test('a reset while the manager is still loading leaves no manager behind', async () => {
  const { log, stage, app, host } = setup();
  hallApp(app);
  const p = host.get();
  host.reset();
  const got = await p;
  assert.equal(got, null);
  assert.equal(stage.nav, null);
  assert.ok(log.made === 0 || log.disposed >= log.made, 'a manager that did get built was disposed');
});

test('get is idempotent between resets', async () => {
  const { log, app, host } = setup();
  hallApp(app);
  const [a, b] = await Promise.all([host.get(), host.get()]);
  assert.equal(a, b);
  assert.equal(log.made, 1);
});

test('paused manager ignores updates and consumes input', async () => {
  const { app, host } = setup();
  hallApp(app);
  const nav = await host.get();
  nav.pause(true);
  nav.update(0.016);
  nav.pause(false);
  nav.update(0.016);
  assert.equal(typeof nav.paused, 'boolean');
});

// ndc of the hall box corners through a pose must sit inside the safe rectangle
test('after garden then hall the hall top pose fits the hall inside the safe rect', async () => {
  const { log, stage, app, host } = setup();
  const safe = { left: 0.175, bottom: 0.078, top: 0.02 };
  host.setSafe(safe);
  gardenApp(app, stage);
  stage.rig.safe = { left: 0, bottom: 0, top: 0.5 };
  await host.get();
  host.reset();
  hallApp(app);
  await host.get();
  const ctx = log.ctxs[1];
  const pose = ctx.presetPose('top');
  const L = app.layout;
  const bb = { x0: L.xmin, x1: L.xmin + L.W, z0: L.zmin, z1: L.zmin + L.D, bottom: 0, top: 2.6 };
  let x0 = 9; let x1 = -9; let y0 = 9; let y1 = -9;
  for (const x of [bb.x0, bb.x1]) for (const y of [0, 2.6]) for (const z of [bb.z0, bb.z1]) {
    const n = project(pose, { x, y, z }, 16 / 9);
    x0 = Math.min(x0, n.x); x1 = Math.max(x1, n.x); y0 = Math.min(y0, n.y); y1 = Math.max(y1, n.y);
  }
  const eps = 1e-6;
  assert.ok(x0 >= -1 + 2 * safe.left - eps && x1 <= 1 - 2 * (safe.right || 0) + eps);
  assert.ok(y0 >= -1 + 2 * safe.bottom - eps && y1 <= 1 - 2 * safe.top + eps);
});
