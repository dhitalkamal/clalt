// the pure helpers of the field notebook look: state stamps and account chips
import test from 'node:test';
import assert from 'node:assert/strict';
import { stampInfo, chipClass, toneFlag } from '../notebook.js';

test('stampInfo maps a state to a stamp class and plain text', () => {
  assert.deepEqual(stampInfo('busy'), { cls: 'stamp s-busy', text: 'working' });
  assert.deepEqual(stampInfo('idle'), { cls: 'stamp s-idle', text: 'idle' });
  assert.deepEqual(stampInfo('needs-you'), { cls: 'stamp s-need', text: 'needs you' });
});

test('stampInfo keeps a given text and fixes the needs-you spelling', () => {
  assert.equal(stampInfo('busy', 'finishing').text, 'finishing');
  assert.equal(stampInfo('needs-you', 'needs-you (permission prompt)').text, 'needs you (permission prompt)');
  assert.equal(stampInfo('ended', 'ended').cls, 'stamp s-idle');
  assert.equal(stampInfo(undefined).text, 'idle');
});

test('chipClass is stable and falls back to one of three tints', () => {
  assert.equal(chipClass('Work'), 'chip a-work');
  assert.equal(chipClass('personal'), 'chip a-personal');
  assert.equal(chipClass('CLIENT'), 'chip a-client');
  const other = chipClass('side-gig');
  assert.match(other, /^chip a-(work|personal|client)$/);
  assert.equal(chipClass('side-gig'), other);
  assert.equal(chipClass(''), 'chip a-work');
});

test('toneFlag gives a stamp only for the tones that are not a severity', () => {
  assert.deepEqual(toneFlag('error'), { cls: 'flag error', text: 'error' });
  assert.deepEqual(toneFlag('clean'), { cls: 'flag clean', text: 'clean' });
  assert.deepEqual(toneFlag('skipped'), { cls: 'flag skipped', text: 'skipped' });
  assert.equal(toneFlag('high'), null);
});
