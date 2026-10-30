// the View section of settings: the switches that used to sit at the bottom of the screen.
import test from 'node:test';
import assert from 'node:assert/strict';
import { viewRows } from '../viewsection.js';

const base = { labels: true, lines: false, tags: false, spin: false, cam: 'sky', reduced: false };

test('the rows come in a fixed order and mirror the state', () => {
  const rows = viewRows(base);
  assert.deepEqual(rows.map((r) => r.id), ['labels', 'lines', 'tags', 'spin', 'ground']);
  assert.deepEqual(rows.map((r) => r.on), [true, false, false, false, false]);
  assert.ok(rows.every((r) => typeof r.label === 'string' && r.label));
});

test('rows with a key say which one', () => {
  const keys = Object.fromEntries(viewRows(base).map((r) => [r.id, r.key]));
  assert.deepEqual(keys, { labels: 'L', lines: '', tags: '', spin: 'P', ground: 'G' });
});

test('ground is on while the walking camera is', () => {
  assert.equal(viewRows({ ...base, cam: 'ground' }).find((r) => r.id === 'ground').on, true);
});

test('reduced motion turns auto-spin off and says why', () => {
  const spin = viewRows({ ...base, spin: true, reduced: true }).find((r) => r.id === 'spin');
  assert.equal(spin.on, false);
  assert.equal(spin.disabled, true);
  assert.match(spin.note, /less motion/);
  assert.equal(viewRows(base).find((r) => r.id === 'spin').disabled, false);
});
