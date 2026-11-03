import test from 'node:test';
import assert from 'node:assert/strict';
import { createState } from '../state.js';
import { catalog, baseLayout } from './fixtures.mjs';

// applyLayout whose calls finish only when the test says so
function slowApply() {
  const calls = [];
  const fn = (l) => new Promise((resolve) => calls.push({ l, resolve }));
  return { fn, calls };
}
const tick = () => new Promise((r) => setTimeout(r, 0));

function make(apply) {
  return createState({ layout: baseLayout(), catalog, applyLayout: apply });
}

test('previews never overlap and only the latest layout is applied after a slow one', async () => {
  const a = slowApply();
  const ed = make(a.fn);
  ed.layout.title = 'one';
  ed.preview();
  ed.layout.title = 'two';
  ed.preview();
  ed.layout.title = 'three';
  ed.preview();
  assert.equal(a.calls.length, 1);
  a.calls[0].resolve();
  await tick();
  assert.equal(a.calls.length, 2);
  assert.equal(a.calls[1].l.title, 'three');
  a.calls[1].resolve();
  await tick();
  assert.equal(a.calls.length, 2);
});

test('nothing is applied after dispose', async () => {
  const a = slowApply();
  const ed = make(a.fn);
  ed.preview();
  ed.preview();
  ed.dispose();
  a.calls[0].resolve();
  await tick();
  assert.equal(a.calls.length, 1);
  ed.preview();
  ed.changed();
  await tick();
  assert.equal(a.calls.length, 1);
});

test('undo clears a drag in progress', () => {
  const ed = make(async () => {});
  ed.edit((l) => (l.title = 'x'));
  ed.drag = { type: 'move-room' };
  ed.dragRect = { x: 0, z: 0, w: 2, d: 2 };
  ed.undo();
  assert.equal(ed.drag, null);
  assert.equal(ed.dragRect, null);
  ed.dispose();
});

test('a status can carry actions, and a plain one clears them', () => {
  const ed = make(async () => {});
  const run = () => {};
  ed.setStatus('changed', 'bad', [{ text: 'Load theirs', run }]);
  assert.deepEqual(ed.status.actions, [{ text: 'Load theirs', run }]);
  ed.setStatus('Saved.', 'ok');
  assert.deepEqual(ed.status.actions, []);
});
