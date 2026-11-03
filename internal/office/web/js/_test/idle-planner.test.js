// the planner follows the idle rules: desk and hide agents are never offered a break, team members may be
import test from 'node:test';
import assert from 'node:assert/strict';
import { planBreaks } from '../planner.js';
import { DEFAULT_RULES } from '../idle/resolve.js';
import { makeGrid } from '../scene/nav.js';

const L = { groups: { cooler: [{ name: 'c1' }, { name: 'c2' }, { name: 'c3' }], counter: [{ name: 'k1' }] } };
const agent = (role, state, extra = {}, slot = { kind: 'desk' }) => ({ s: { role, state, seed: 1, leaving: false, account: 'a1', ...extra }, slot, mode: 'seated', nextBreakAt: 0, plan: null, planOffer: null, breakCount: 0 });
const world = (agents, rules, more = {}) => ({ layout: L, agents: new Map(agents.map((a, i) => ['a' + i, a])), time: 100, pairCycle: 0, idleRules: rules, ...more });
const rules = (o) => ({ ...DEFAULT_RULES, accounts: {}, teams: {}, subtypes: {}, ...o });

test('a session set to stay at the desk is not offered a break', () => {
  const a = agent('main', 'idle');
  assert.equal(planBreaks(world([a], rules({ accounts: { a1: 'desk' } })), true).length, 0);
  assert.equal(a.planOffer, null);
});

test('a hidden agent is not offered a break either', () => {
  assert.equal(planBreaks(world([agent('sub', 'idle')], rules({ default: 'hide' })), true).length, 0);
});

test('team members stay at the desk by default and take breaks when asked', () => {
  const t = () => agent('team', 'idle', { team: 'qa' }, { kind: 'desk', node: {}, chair: {} });
  assert.equal(planBreaks(world([t()], undefined), true).length, 0);
  const a = t();
  assert.equal(planBreaks(world([a], rules({ teams: { qa: 'break' } })), true).length, 1);
  assert.ok(a.planOffer);
  const b = t();
  assert.equal(planBreaks(world([b], rules({ teamDefault: 'break' })), true).length, 1);
});

test('a team member without a usable seat stays at the desk even when asked to break', () => {
  const t = agent('team', 'idle', { team: 'qa' }, { kind: 'desk' });
  assert.equal(planBreaks(world([t], rules({ teamDefault: 'break' })), true).length, 0);
  const standing = agent('team', 'idle', { team: 'qa' }, { kind: 'stand', node: {}, chair: {} });
  assert.equal(planBreaks(world([standing], rules({ teamDefault: 'break' })), true).length, 0);
});

test('busy and needs-you team members never break whatever the rules say', () => {
  const list = [agent('team', 'busy', { team: 'qa' }, { kind: 'desk', node: {}, chair: {} }), agent('team', 'needs-you', { team: 'qa' }, { kind: 'desk', node: {}, chair: {} })];
  assert.equal(planBreaks(world(list, rules({ teamDefault: 'break' })), true).length, 0);
});

test('an offer made before the rule changed to desk is withdrawn', () => {
  const a = agent('main', 'idle');
  a.planOffer = { steps: [{ spot: 'c1' }], carry: false };
  planBreaks(world([a], rules({ default: 'desk' })));
  assert.equal(a.planOffer, null);
});

// a hall with one room r1 and open floor
function hall() {
  const g = makeGrid(10, 10);
  g.open.fill(1);
  return { ...L, rooms: [{ id: 'r1', level: 0, x: 0, z: 0, w: 4, d: 4 }], grids: [g, makeGrid(10, 10)], spots: {}, X: (c) => c, Z: (c) => c, node: (level, p) => ({ level, ...p }), flood: { has: () => true } };
}
const layoutJson = { v: 1, rooms: [{ id: 'r1', type: 'dev', label: 'Dev' }] };

test('a room choice offers a rest plan in that room instead of a break', () => {
  const a = agent('main', 'idle');
  const app = world([a], rules({ default: 'room:r1' }), { layout: hall(), layoutJson });
  assert.equal(planBreaks(app, true).length, 1);
  const step = a.planOffer.steps[0];
  assert.equal(step.act, 'rest');
  assert.ok(app.layout.spots[step.spot], 'the floor spot is registered');
});

test('two agents resting in one room get different cells', () => {
  const a = agent('main', 'idle');
  const b = agent('main', 'idle');
  planBreaks(world([a, b], rules({ default: 'room:r1' }), { layout: hall(), layoutJson }), true);
  assert.notEqual(a.planOffer.steps[0].spot, b.planOffer.steps[0].spot);
});

test('a room with no free place falls back to a break', () => {
  const a = agent('main', 'idle');
  const lay = hall();
  lay.grids[0].open.fill(0);
  planBreaks(world([a], rules({ default: 'room:r1' }), { layout: lay, layoutJson }), true);
  assert.ok(a.planOffer, 'got a plan');
  assert.notEqual(a.planOffer.steps[0].act, 'rest');
});

test('a room choice in a garden is a break', () => {
  const a = agent('main', 'idle');
  planBreaks(world([a], rules({ default: 'room:r1' }), { layoutJson: { v: 2, rooms: layoutJson.rooms } }), true);
  assert.ok(a.planOffer);
  assert.notEqual(a.planOffer.steps[0].act, 'rest');
});
