// the key table of the page
import test from 'node:test';
import assert from 'node:assert/strict';
import { actionFor } from '../keymap.js';
import { parseHash } from '../viewstate.js';

const ev = (key, extra = {}) => ({ key, target: { tagName: 'BODY' }, ...extra });

test('6 to 9 and 1 to 5 are free, the preset keys are gone', () => {
  for (const k of ['1', '2', '5', '6', '7', '8', '9']) assert.equal(actionFor(ev(k)), null);
});

test('mode and toggle keys', () => {
  assert.deepEqual(actionFor(ev('g')), { type: 'ground' });
  assert.deepEqual(actionFor(ev('v')), { type: 'view' });
  assert.deepEqual(actionFor(ev('f')), { type: 'follow' });
  assert.deepEqual(actionFor(ev('p')), { type: 'spin' });
  assert.deepEqual(actionFor(ev('l')), { type: 'labels' });
  assert.deepEqual(actionFor(ev('Home')), { type: 'home' });
  assert.deepEqual(actionFor(ev('Escape')), { type: 'escape' });
});

test('camera movement keys are not page actions, typing and chords are ignored', () => {
  for (const k of ['w', 'a', 's', 'd', 'q', 'e', 'r', 'c']) assert.equal(actionFor(ev(k)), null);
  assert.equal(actionFor({ key: 'g', target: { tagName: 'INPUT' } }), null);
  assert.equal(actionFor(ev('g', { ctrlKey: true })), null);
});

test('old camera ids in a url still resolve', () => {
  assert.equal(parseHash('#cam=eye').cam, 'ground');
  assert.equal(parseHash('#cam=orbit').cam, 'sky');
  for (const c of ['top', 'plan', 'iso', 'hero']) assert.equal(parseHash('#cam=' + c).cam, 'sky');
});

test('auto-repeat does not fire toggles or presets', () => {
  for (const k of ['l', 'p', 'g', 'v', 'f', '6', '2', 'Escape']) assert.equal(actionFor(ev(k, { repeat: true })), null);
  assert.deepEqual(actionFor(ev('l', { repeat: false })), { type: 'labels' });
});
