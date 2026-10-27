// url hash, remembered pick and key map. pure, no dom.
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseHash, buildHash, CAMS } from '../viewstate.js';
import { actionFor } from '../keymap.js';

test('camera presets and their keys', () => {
  assert.deepEqual(CAMS.map((c) => c.id), ['top', 'plan', 'iso', 'hero', 'ground', 'sky']);
  assert.deepEqual(CAMS.map((c) => c.key), ['6', '7', '8', '9', 'g', '']);
});

const key = (k, extra = {}) => ({ key: k, ctrlKey: false, metaKey: false, altKey: false, target: { tagName: 'BODY' }, ...extra });

test('letter keys pick cameras, u spins, l toggles labels', () => {
    assert.deepEqual(actionFor(key('6')), { type: 'cam', id: 'top' });
  assert.deepEqual(actionFor(key('G')), { type: 'ground' });
  assert.deepEqual(actionFor(key('p')), { type: 'spin' });
  assert.deepEqual(actionFor(key('l')), { type: 'labels' });
});

test('keys are ignored with modifiers, in text fields and while editing', () => {
    assert.equal(actionFor(key('q', { ctrlKey: true })), null);
  assert.equal(actionFor(key('q', { metaKey: true })), null);
  assert.equal(actionFor(key('q', { target: { tagName: 'INPUT' } })), null);
  assert.equal(actionFor(key('q', { target: { tagName: 'DIV', isContentEditable: true } })), null);
  assert.equal(actionFor(key('q'), { editing: true }), null);
});

test('a template switch never keeps ground as the camera', async () => {
  const { camAfterSwitch } = await import('../viewstate.js');
  assert.equal(camAfterSwitch('ground'), 'sky');
  assert.equal(camAfterSwitch('iso'), 'iso');
  assert.equal(camAfterSwitch('sky'), 'sky');
});

test('parseHash reads the camera only', () => {
  assert.deepEqual(parseHash('#cam=hero'), { cam: 'hero' });
  assert.deepEqual(parseHash('#template=river&cam=nope'), { cam: null });
  assert.equal(buildHash({ cam: 'ground' }), '#cam=ground');
});

test('number keys 1 to 5 do nothing', () => {
  for (const k of ['1', '2', '3', '4', '5']) assert.equal(actionFor(key(k)), null);
  assert.equal(actionFor(key('6')).type, 'cam');
});
