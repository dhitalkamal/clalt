// the brief walk and the asking-for-help modes, driven on duck typed agents over the compiled hall
import test from 'node:test';
import assert from 'node:assert/strict';
import { hallCompiled } from './hall-fixture.js';
import { installMove } from '../agentMove.js';
import { installBreaks } from '../breaks.js';
import { installBrief } from '../agentBrief.js';
import { installDesk } from '../agentDesk.js';

const stub = { reset() { return this; }, play() {}, crossFadeFrom() {} };
const quat = { quaternion: { copy() {} } };

class Fake {}
installMove(Fake);
installBreaks(Fake);
installBrief(Fake);
installDesk(Fake);

function fake(slot, L, env, over = {}) {
  const a = new Fake();
  Object.assign(a, {
    mode: 'seated', slot, L, env, path: null, plan: null, planOffer: null, curNode: slot.node, s: { id: 's', state: 'busy' }, now: 10,
    model: { parent: null, position: { x: slot.x, y: slot.y, z: slot.z, set(x, y, z) { Object.assign(this, { x, y, z }); } },
      rotation: { y: 0, set(x, y) { this.y = y; } }, scale: { setScalar() {} } },
    world: { add() {} }, seat: { add() {} }, bones: { head: quat, torso: quat }, base: { head: {}, torso: {} },
    actions: new Proxy({}, { get: () => stub }), playing: null, mixer: { stopAllAction() {} },
    cup: {}, paper: {}, book: {}, paddle: {}, nextBreakAt: 999,
  }, over);
  return a;
}

function office() {
  const L = hallCompiled();
  const desks = L.items.filter((it) => it.def.id === 'desk' && it.level === 0 && L.seatSlot(it));
  const env = { agents: new Map(), glance: null };
  const main = fake(L.seatSlot(desks[0]), L, env, { s: { id: 'm', state: 'busy' } });
  const sub = fake(L.seatSlot(desks.at(-1)), L, env, { s: { id: 'm/1', state: 'busy' } });
  env.agents.set('m', main);
  env.agents.set('m/1', sub);
  return { L, env, main, sub, desks };
}

const finish = (a) => { while (a.mode === 'walking') { const p = a.path; a.path = null; p.done(); } };
const setCalm = (on) => { globalThis.matchMedia = on ? () => ({ matches: true }) : undefined; };

test('a queued brief makes the seated main walk to the new subagent desk', () => {
  setCalm(false);
  const { main, sub } = office();
  main.queueBrief('m/1');
  main.tickDesk(10, false);
  assert.equal(main.mode, 'walking');
  assert.equal(main.path.kind, 'brief');
  const pts = main.path.fn();
  const end = pts.at(-1);
  assert.ok(pts.length > 1);
  assert.equal(main.path.dest(main.L, main) === sub.slot.corr, true);
  assert.ok(end);
});

test('on arrival the main faces the desk, briefs for about two seconds, then walks back and sits', () => {
  setCalm(false);
  const { main, sub } = office();
  main.queueBrief('m/1');
  main.tickDesk(10, false);
  finish(main);
  assert.equal(main.mode, 'leisure');
  assert.equal(main.act, 'brief');
  assert.ok(main.actEnd - main.actStart >= 1.5 && main.actEnd - main.actStart <= 2.5);
  main.now = main.actEnd - 0.1;
  main.tickLeisure(main.now);
  assert.equal(main.mode, 'leisure');
  main.now = main.actEnd + 0.1;
  main.tickLeisure(main.now);
  assert.equal(main.mode, 'walking');
  assert.equal(main.path.kind, 'back');
  const pts = main.path.fn();
  const goal = main.slot.node;
  assert.ok(Math.hypot(pts.at(-1).x - goal.x, pts.at(-1).z - goal.z) < 1e-6);
  finish(main);
  assert.equal(main.mode, 'seated');
  assert.equal(main.briefQueue.length, 0);
  assert.ok(sub);
});

test('no brief starts for a main that is walking, needs you, leaving or has no queue', () => {
  setCalm(false);
  for (const over of [{ mode: 'walking' }, { s: { id: 'm', state: 'needs-you' } }, { s: { id: 'm', state: 'busy', leaving: true } }]) {
    const { main } = office();
    Object.assign(main, over);
    main.queueBrief('m/1');
    main.tickDesk(10, !!over.s?.leaving);
    assert.notEqual(main.path && main.path.kind, 'brief');
  }
  const { main } = office();
  main.tickDesk(10, false);
  assert.equal(main.mode, 'seated');
});

test('a brief whose subagent has no character is dropped without a walk', () => {
  setCalm(false);
  const { main, env } = office();
  main.queueBrief('m/ghost');
  main.tickDesk(10, false);
  assert.equal(main.mode, 'seated');
  assert.equal(main.briefQueue.length, 0);
  assert.ok(env);
});

test('needs-you during the walk to the desk sends the main back to sit, then it stands', () => {
  setCalm(false);
  const { main } = office();
  main.queueBrief('m/1');
  main.tickDesk(10, false);
  main.model.position.x += 1;
  main.s.state = 'needs-you';
  main.tickDesk(10.1, false);
  assert.equal(main.mode, 'walking');
  assert.equal(main.path.kind, 'back');
  finish(main);
  assert.equal(main.mode, 'seated');
  main.tickDesk(10.2, false);
  assert.ok(['walking', 'standing'].includes(main.mode));
});

test('needs-you during the briefing ends it and walks back at once', () => {
  setCalm(false);
  const { main } = office();
  main.queueBrief('m/1');
  main.tickDesk(10, false);
  finish(main);
  main.s.state = 'needs-you';
  main.now = main.actStart + 0.5;
  main.tickLeisure(main.now);
  assert.equal(main.mode, 'walking');
  assert.equal(main.path.kind, 'back');
});

test('with reduced motion a brief is never queued or started', () => {
  setCalm(true);
  const { main } = office();
  main.queueBrief('m/1');
  main.tickDesk(10, false);
  assert.equal(main.mode, 'seated');
  assert.ok(!main.briefQueue || main.briefQueue.length === 0);
  setCalm(false);
});

test('needs-you with stand points walks to stand, and with reduced motion stands in place', () => {
  setCalm(false);
  let o = office();
  o.main.slot = { ...o.main.slot, stand: [{ x: 1, z: 1 }] };
  o.main.s.state = 'needs-you';
  o.main.tickDesk(10, false);
  assert.equal(o.main.mode, 'walking');
  assert.equal(o.main.path.kind, 'stand');
  setCalm(true);
  o = office();
  o.main.slot = { ...o.main.slot, stand: [{ x: 1, z: 1 }] };
  o.main.s.state = 'needs-you';
  o.main.tickDesk(10, false);
  assert.equal(o.main.mode, 'standing');
  setCalm(false);
});

test('answered: the standing agent walks back normally and sits at once with reduced motion', () => {
  const slot = (o) => ({ ...o.main.slot, kind: 'desk', stand: [{ x: 1, z: 1 }] });
  setCalm(false);
  let o = office();
  Object.assign(o.main, { mode: 'standing', slot: slot(o) });
  o.main.tickDesk(10, false);
  assert.equal(o.main.mode, 'walking');
  assert.equal(o.main.path.kind, 'back');
  setCalm(true);
  o = office();
  Object.assign(o.main, { mode: 'standing', slot: slot(o) });
  o.main.tickDesk(10, false);
  assert.equal(o.main.mode, 'seated');
  setCalm(false);
});

test('a seat with no stand points stays seated while needs-you', () => {
  setCalm(false);
  const { main } = office();
  main.slot = { ...main.slot, stand: [] };
  main.s.state = 'needs-you';
  main.tickDesk(10, false);
  assert.equal(main.mode, 'seated');
});

test('the main waits at its desk until the new subagent has sat down, then walks over', () => {
  setCalm(false);
  const { main, sub } = office();
  sub.mode = 'walking';
  main.queueBrief('m/1');
  main.tickDesk(10, false);
  assert.equal(main.mode, 'seated', 'no brief to an empty chair');
  sub.mode = 'seated';
  main.tickDesk(11, false);
  assert.equal(main.mode, 'walking');
  assert.equal(main.path.kind, 'brief');
});
