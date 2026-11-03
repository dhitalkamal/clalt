// who briefs whom when a subagent appears, and the raised-hand maths, as pure functions
import test from 'node:test';
import assert from 'node:assert/strict';
import { subIdsOf, newBriefs, briefReady } from '../behave/brief.js';
import { canStand, seatedAsk, askPose } from '../behave/ask.js';

const sess = (id, subs) => ({ id, leaving: false, subs: subs.map((s) => (typeof s === 'string' ? { id: s, leaving: false } : s)) });

test('subIdsOf lists live subagents per session and skips leaving ones', () => {
  const m = subIdsOf([sess('a', ['a/1', { id: 'a/2', leaving: true }]), sess('b', []), { id: 'c', leaving: true, subs: [{ id: 'c/1' }] }]);
  assert.deepEqual([...m.get('a')], ['a/1']);
  assert.deepEqual([...m.get('b')], []);
  assert.equal(m.has('c'), false);
});

test('a new subagent of a known session is briefed by that session', () => {
  const prev = subIdsOf([sess('a', ['a/1'])]);
  const cur = subIdsOf([sess('a', ['a/1', 'a/2'])]);
  assert.deepEqual(newBriefs(prev, cur, false), [{ main: 'a', sub: 'a/2' }]);
});

test('nothing is briefed on the first snapshot after load', () => {
  const cur = subIdsOf([sess('a', ['a/1'])]);
  assert.deepEqual(newBriefs(new Map(), cur, true), []);
});

test('a session that appears with its subagents is not briefing, it is walking in', () => {
  const cur = subIdsOf([sess('a', ['a/1'])]);
  assert.deepEqual(newBriefs(new Map(), cur, false), []);
});

test('unchanged and ended subagents start no brief', () => {
  const prev = subIdsOf([sess('a', ['a/1', 'a/2'])]);
  assert.deepEqual(newBriefs(prev, subIdsOf([sess('a', ['a/1'])]), false), []);
  assert.deepEqual(newBriefs(prev, prev, false), []);
});

test('two new subagents give two briefs in order', () => {
  const prev = subIdsOf([sess('a', []), sess('b', ['b/1'])]);
  const cur = subIdsOf([sess('a', ['a/1', 'a/2']), sess('b', ['b/1'])]);
  assert.deepEqual(newBriefs(prev, cur, false), [{ main: 'a', sub: 'a/1' }, { main: 'a', sub: 'a/2' }]);
});

const ready = (over = {}) => ({ mode: 'seated', s: { state: 'busy' }, briefQueue: [{ sub: 'x' }], path: null, ...over });

test('briefReady needs a seated main with a queued brief', () => {
  assert.equal(briefReady(ready(), false), true);
  assert.equal(briefReady(ready({ briefQueue: [] }), false), false);
  assert.equal(briefReady(ready({ briefQueue: null }), false), false);
});

test('briefReady is false when needs-you, walking, leaving, ended or on a break', () => {
  assert.equal(briefReady(ready({ s: { state: 'needs-you' } }), false), false);
  assert.equal(briefReady(ready({ s: { state: 'busy', leaving: true } }), false), false);
  assert.equal(briefReady(ready({ s: { state: 'ended' } }), false), false);
  assert.equal(briefReady(ready({ mode: 'walking' }), false), false);
  assert.equal(briefReady(ready({ mode: 'leisure' }), false), false);
});

test('briefReady is false with reduced motion', () => assert.equal(briefReady(ready(), true), false));

test('canStand needs at least one stand point', () => {
  assert.equal(canStand({ stand: [{ x: 0, z: 0 }] }), true);
  assert.equal(canStand({ stand: [] }), false);
  assert.equal(canStand({ stand: null }), false);
  assert.equal(canStand({}), false);
  assert.equal(canStand(null), false);
});

test('seatedAsk is true for a needs-you agent at a seat with no stand points only', () => {
  assert.equal(seatedAsk('seated', 'needs-you', { stand: [] }), true);
  assert.equal(seatedAsk('seated', 'needs-you', { stand: [{ x: 1, z: 1 }] }), false);
  assert.equal(seatedAsk('seated', 'busy', { stand: [] }), false);
  assert.equal(seatedAsk('standing', 'needs-you', { stand: [] }), false);
});

test('askPose swings and bounces, and holds still with reduced motion', () => {
  const a = askPose(0.1, false);
  assert.notEqual(a.arm, askPose(0.3, false).arm);
  assert.ok(a.lift > 0);
  for (const t of [0, 0.1, 0.37, 2]) {
    const c = askPose(t, true);
    assert.deepEqual(c, { arm: -1.35, lift: 0, lean: 0 });
  }
});
