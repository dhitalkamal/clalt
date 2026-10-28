import test from 'node:test';
import assert from 'node:assert/strict';
import { limitTone, meterPct } from '../summary/format.js';

test('limitTone: amber above 60, red above 90', () => {
  assert.equal(limitTone(0), 'ok');
  assert.equal(limitTone(30), 'ok');
  assert.equal(limitTone(60), 'ok');
  assert.equal(limitTone(61), 'warn');
  assert.equal(limitTone(75), 'warn');
  assert.equal(limitTone(90), 'warn');
  assert.equal(limitTone(95), 'hot');
});

test('meterPct clamps to 0..100 and drops junk', () => {
  assert.equal(meterPct(42.4), 42);
  assert.equal(meterPct(140), 100);
  assert.equal(meterPct(-3), 0);
  assert.equal(meterPct(NaN), 0);
  assert.equal(meterPct(null), 0);
});
