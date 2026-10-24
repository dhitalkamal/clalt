// the key table of the page
import test from 'node:test';
import assert from 'node:assert/strict';
import { actionFor } from '../keymap.js';
import { parseHash } from '../viewstate.js';

const ev = (key, extra = {}) => ({ key, target: { tagName: 'BODY' }, ...extra });
const tpls = ['hall', 'rows', 'village', 'river', 'street'].map((id) => ({ id }));

test('1 to 5 pick templates, 6 to 9 pick camera presets', () => {
  assert.deepEqual(actionFor(ev('2'), tpls), { type: 'template', id: 'rows' });
  assert.deepEqual(actionFor(ev('6'), tpls), { type: 'cam', id: 'top' });
  assert.deepEqual(actionFor(ev('7'), tpls), { type: 'cam', id: 'plan' });
  assert.deepEqual(actionFor(ev('8'), tpls), { type: 'cam', id: 'iso' });
  assert.deepEqual(actionFor(ev('9'), tpls), { type: 'cam', id: 'hero' });
});

test('mode and toggle keys', () => {
  assert.deepEqual(actionFor(ev('g'), tpls), { type: 'ground' });
  assert.deepEqual(actionFor(ev('v'), tpls), { type: 'view' });
  assert.deepEqual(actionFor(ev('f'), tpls), { type: 'follow' });
  assert.deepEqual(actionFor(ev('p'), tpls), { type: 'spin' });
  assert.deepEqual(actionFor(ev('l'), tpls), { type: 'labels' });
  assert.deepEqual(actionFor(ev('Home'), tpls), { type: 'home' });
  assert.deepEqual(actionFor(ev('Escape'), tpls), { type: 'escape' });
});

test('camera movement keys are not page actions, typing and chords are ignored', () => {
  for (const k of ['w', 'a', 's', 'd', 'q', 'e', 'r', 'c']) assert.equal(actionFor(ev(k), tpls), null);
  assert.equal(actionFor({ key: 'g', target: { tagName: 'INPUT' } }, tpls), null);
  assert.equal(actionFor(ev('g', { ctrlKey: true }), tpls), null);
});

test('old camera ids in a url still resolve', () => {
  assert.equal(parseHash('#cam=eye').cam, 'ground');
  assert.equal(parseHash('#cam=orbit').cam, 'sky');
  assert.equal(parseHash('#cam=hero').cam, 'hero');
});

test('auto-repeat does not fire toggles or presets', () => {
  for (const k of ['l', 'p', 'g', 'v', 'f', '6', '2', 'Escape']) assert.equal(actionFor(ev(k, { repeat: true }), tpls), null);
  assert.deepEqual(actionFor(ev('l', { repeat: false }), tpls), { type: 'labels' });
});
