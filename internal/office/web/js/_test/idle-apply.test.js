// the per-frame pass: hide idle agents that are set to hide, call resting agents back when the choice changes
import test from 'node:test';
import assert from 'node:assert/strict';
import { applyIdle } from '../idle/apply.js';
import { DEFAULT_RULES } from '../idle/resolve.js';

const rules = (o) => ({ ...DEFAULT_RULES, accounts: {}, teams: {}, subtypes: {}, ...o });
const agent = (state, extra = {}, plan = null) => {
  const a = { s: { role: 'main', account: 'a1', state, ...extra }, plan, abort: false, hidden: false };
  a.setHidden = (v) => { a.hidden = v; };
  return a;
};
const app = (agents, r, layoutJson = { v: 1, rooms: [{ id: 'r1', type: 'dev' }] }) => ({ agents: new Map(agents.map((a, i) => ['a' + i, a])), idleRules: r, layoutJson });

test('an idle agent set to hide is hidden, and shows again as soon as it works', () => {
  const a = agent('idle');
  const w = app([a], rules({ default: 'hide' }));
  applyIdle(w);
  assert.equal(a.hidden, true);
  a.s.state = 'busy';
  applyIdle(w);
  assert.equal(a.hidden, false);
  a.s.state = 'needs-you';
  applyIdle(w);
  assert.equal(a.hidden, false);
});

test('desk and break agents are never hidden', () => {
  const a = agent('idle');
  applyIdle(app([a], rules({ default: 'desk' })));
  assert.equal(a.hidden, false);
  applyIdle(app([a], undefined));
  assert.equal(a.hidden, false);
});

test('a resting agent is called back when it gets busy or the choice moves on', () => {
  const plan = { steps: [], rest: 'r1' };
  const stays = agent('idle', {}, plan);
  applyIdle(app([stays], rules({ default: 'room:r1' })));
  assert.equal(stays.abort, false);
  const busy = agent('busy', {}, plan);
  applyIdle(app([busy], rules({ default: 'room:r1' })));
  assert.equal(busy.abort, true);
  const moved = agent('idle', {}, plan);
  applyIdle(app([moved], rules({ default: 'break' })));
  assert.equal(moved.abort, true);
});

test('a normal break is left alone', () => {
  const a = agent('idle', {}, { steps: [] });
  applyIdle(app([a], rules({ default: 'desk' })));
  assert.equal(a.abort, false);
});
