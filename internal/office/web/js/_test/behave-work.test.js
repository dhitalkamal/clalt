// the pure mapping from a snapshot's doing field to what a seated agent acts out
import test from 'node:test';
import assert from 'node:assert/strict';
import { track, currentAct, REACT_MS } from '../behave/work.js';
import { LOOK } from '../behave/look.js';

const d = (kind, state, at = '2026-10-31T10:00:00Z', tool = 'X') => ({ kind, tool, state, at });
const act = (doing, now = 0, prev) => currentAct(track(prev, doing, now), now);

test('read, edit and web map to reading, typing and browsing', () => {
  assert.equal(act(d('read', 'running')), 'read');
  assert.equal(act(d('read', 'ok')), 'read');
  assert.equal(act(d('edit', 'running')), 'type');
  assert.equal(act(d('web', 'running')), 'web');
});

test('a running command waits, a failed one is a facepalm', () => {
  assert.equal(act(d('run', 'running')), 'wait');
  assert.equal(act(d('run', 'fail')), 'fail');
});

test('no work, or a tool with no act, leaves the plain busy pose', () => {
  assert.equal(act(null), null);
  assert.equal(act(undefined), null);
  assert.equal(act(d('other', 'running')), null);
  assert.equal(act({ kind: 'nonsense', state: 'running', at: 'x' }), null);
});

test('a command that ends ok only cheers when it was seen waiting', () => {
  assert.equal(act(d('run', 'ok')), null);
  const waiting = track(undefined, d('run', 'running', 'a'), 0);
  assert.equal(currentAct(track(waiting, d('run', 'ok', 'b'), 1000), 1000), 'pass');
});

test('fail and pass last a short moment, then the pose goes back to plain busy', () => {
  const waiting = track(undefined, d('run', 'running', 'a'), 0);
  const failed = track(waiting, d('run', 'fail', 'b'), 1000);
  assert.equal(currentAct(failed, 1000 + REACT_MS - 1), 'fail');
  assert.equal(currentAct(failed, 1000 + REACT_MS + 1), null);
  const passed = track(waiting, d('run', 'ok', 'b'), 1000);
  assert.equal(currentAct(passed, 1000 + REACT_MS - 1), 'pass');
  assert.equal(currentAct(passed, 1000 + REACT_MS + 1), null);
});

test('the same event seen again on the next poll does not restart the moment', () => {
  const first = track(undefined, d('run', 'fail', 'b'), 0);
  const again = track(first, d('run', 'fail', 'b'), REACT_MS + 500);
  assert.equal(currentAct(again, REACT_MS + 500), null);
});

test('a new event after a reaction replaces it at once', () => {
  const failed = track(undefined, d('run', 'fail', 'b'), 0);
  const next = track(failed, d('edit', 'running', 'c'), 500);
  assert.equal(currentAct(next, 500), 'type');
});

test('doing going away clears the act and forgets the wait', () => {
  const waiting = track(undefined, d('run', 'running', 'a'), 0);
  const cleared = track(waiting, null, 100);
  assert.equal(currentAct(cleared, 100), null);
  assert.equal(currentAct(track(cleared, d('run', 'ok', 'b'), 200), 200), null);
});

test('every act has a bubble icon and a screen glow colour', () => {
  for (const a of ['read', 'type', 'wait', 'web', 'pass', 'fail']) {
    assert.equal(typeof LOOK[a].icon, 'string', a);
    assert.equal(typeof LOOK[a].glow, 'number', a);
  }
});
