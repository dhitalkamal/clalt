// picture cards in the settings dialog: one small line sketch per office template, a plain one for saved layouts.
import test from 'node:test';
import assert from 'node:assert/strict';
import { templateSketch } from '../settings.js';

test('every built-in template has its own sketch', () => {
  const ids = ['rows', 'village', 'river', 'street', 'hall'];
  const paths = ids.map(templateSketch);
  for (const p of paths) assert.match(p, /^M[\d .MLHVhvlmcCsSaAzZ-]+$/);
  assert.equal(new Set(paths).size, ids.length);
});

test('the old default id draws the hall', () => {
  assert.equal(templateSketch('default'), templateSketch('hall'));
});

test('a saved layout or an unknown id gets the plain sketch', () => {
  const plain = templateSketch('my-floor');
  assert.equal(templateSketch(''), plain);
  assert.equal(templateSketch(undefined), plain);
  assert.notEqual(plain, templateSketch('hall'));
});
