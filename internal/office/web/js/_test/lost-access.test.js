// a stale tab says it lost access in plain words, never the raw word unauthorized
import test from 'node:test';
import assert from 'node:assert/strict';
import { LOST_ACCESS, failText, pollErrorText } from '../lostaccess.js';
import { errorText, deleteErrorText, autostartErrorText } from '../settings.js';
import { errorText as activityError } from '../activity/api.js';

test('the banner text names what to do and what is lost', () => {
  assert.match(LOST_ACCESS, /lost its access to clalt office/);
  assert.match(LOST_ACCESS, /open a new link with clalt office open/);
  assert.match(LOST_ACCESS, /unsaved changes here will be lost/);
  assert.doesNotMatch(LOST_ACCESS, /restart/);
});

test('failText maps 401 and keeps other bodies', () => {
  assert.equal(failText(401, 'unauthorized\n'), LOST_ACCESS);
  assert.equal(failText(400, 'title is empty\n'), 'title is empty');
  assert.equal(failText(500, ''), 'http 500');
});

test('the poll banner follows the same wording', () => {
  assert.equal(pollErrorText(new Error('auth')), LOST_ACCESS);
  assert.match(pollErrorText(new Error('http 500')), /lost the connection/);
});

test('save, delete and autostart errors say it plainly on 401', () => {
  assert.equal(errorText({ status: 401, text: 'unauthorized' }), 'could not save the default office: ' + LOST_ACCESS);
  assert.equal(deleteErrorText({ status: 401, text: 'unauthorized' }), 'could not delete the layout: ' + LOST_ACCESS);
  assert.equal(autostartErrorText({ status: 401, text: 'unauthorized' }), 'could not change start at login: ' + LOST_ACCESS);
  assert.equal(activityError(new Error('auth')), LOST_ACCESS);
});
