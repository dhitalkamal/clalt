// the flat ring under an agent takes the colour of its account
import test from 'node:test';
import assert from 'node:assert/strict';
import { ringColor, tintFor, makeSession, makeSub } from '../model.js';

test('a session and its subagent share the account colour', () => {
  const main = makeSession({ handle: 'h1', cwd: '/x' }, 'work');
  const sub = makeSub(main, [], { id: 's1', type: 'Explore' });
  assert.equal(ringColor(main), tintFor('work'));
  assert.equal(ringColor(sub), tintFor('work'));
});

test('two accounts get two colours', () => {
  assert.notEqual(ringColor({ role: 'main', account: 'a1' }), ringColor({ role: 'main', account: 'a2' }));
});

test('team members and agents without an account get no ring', () => {
  assert.equal(ringColor({ role: 'team', account: null }), null);
  assert.equal(ringColor({ role: 'team', account: 'work' }), null);
  assert.equal(ringColor({ role: 'main', account: '' }), null);
  assert.equal(ringColor({ role: 'main' }), null);
  assert.equal(ringColor(null), null);
});
