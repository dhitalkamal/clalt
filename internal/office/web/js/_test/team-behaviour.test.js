// who may take a break, and where a finished intern walks
import test from 'node:test';
import assert from 'node:assert/strict';
import { planBreaks } from '../planner.js';
import { handoffOwner } from '../handoff.js';

const L = { groups: { cooler: [{ name: 'c1' }, { name: 'c2' }, { name: 'c3' }], arcade: [{ name: 'a1' }], bag: [{ name: 'b1' }], counter: [{ name: 'k1' }] } };
const agent = (role, state, extra = {}) => ({ s: { role, state, seed: 1, leaving: false, ...extra }, slot: { kind: 'desk' }, mode: 'seated', nextBreakAt: 0, plan: null, planOffer: null, breakCount: 0 });
const app = (agents) => ({ layout: L, agents: new Map(agents.map((a, i) => ['a' + i, a])), time: 100, pairCycle: 0 });

test('idle sessions and generic subagents get break plans', () => {
  assert.equal(planBreaks(app([agent('main', 'idle')])).length, 1);
  assert.equal(planBreaks(app([agent('sub', 'idle')])).length, 1);
});

test('busy and needs-you agents stay at their desk', () => {
  const list = [agent('main', 'busy'), agent('sub', 'busy'), agent('main', 'needs-you'), agent('team', 'busy')];
  assert.equal(planBreaks(app(list), true).length, 0);
});

test('team members never take breaks, idle or not', () => {
  const list = [agent('team', 'idle'), agent('team', 'idle'), agent('team', 'busy')];
  assert.equal(planBreaks(app(list), true).length, 0);
  for (const a of list) assert.equal(a.planOffer, null);
});

test('interns never wander, they are busy runs', () => {
  const a = agent('sub', 'busy', { intern: true });
  assert.equal(planBreaks(app([a]), true).length, 0);
});

test('a finished intern hands over to the session that started it', () => {
  const owner = { id: 's2' };
  assert.equal(handoffOwner({ role: 'sub', intern: true, owner: 's2', session: { id: 's1' } }), 's2');
  assert.equal(handoffOwner({ role: 'sub', session: owner }), 's2');
});
