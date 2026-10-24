import test from 'node:test';
import assert from 'node:assert/strict';
import { createHistory } from '../history.js';

test('undo and redo walk the snapshots', () => {
  const h = createHistory({ n: 0 });
  h.push({ n: 1 });
  h.push({ n: 2 });
  assert.deepEqual(h.undo(), { n: 1 });
  assert.deepEqual(h.undo(), { n: 0 });
  assert.equal(h.undo(), null);
  assert.deepEqual(h.redo(), { n: 1 });
  assert.deepEqual(h.redo(), { n: 2 });
  assert.equal(h.redo(), null);
});

test('a new push drops the redo branch', () => {
  const h = createHistory({ n: 0 });
  h.push({ n: 1 });
  h.undo();
  h.push({ n: 5 });
  assert.equal(h.canRedo(), false);
  assert.deepEqual(h.undo(), { n: 0 });
});

test('identical snapshots are not recorded twice', () => {
  const h = createHistory({ n: 0 });
  assert.equal(h.push({ n: 0 }), false);
  assert.equal(h.canUndo(), false);
});

test('history is bounded to 100 undo steps', () => {
  const h = createHistory({ n: 0 });
  for (let i = 1; i <= 150; i++) h.push({ n: i });
  let steps = 0;
  while (h.undo()) steps++;
  assert.equal(steps, 100);
  assert.deepEqual(h.current(), { n: 50 });
});

test('snapshots are copies, not references', () => {
  const src = { n: 1, a: [1] };
  const h = createHistory({ n: 0 });
  h.push(src);
  src.a.push(2);
  assert.deepEqual(h.current(), { n: 1, a: [1] });
});
