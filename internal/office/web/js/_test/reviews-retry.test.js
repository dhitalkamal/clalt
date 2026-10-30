// retry state of failed review rows in the pr reviews panel. pure model, no dom.
// the attempt cap comes from reply.maxAttempts, never from a js constant.
process.env.TZ = 'UTC';
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import * as m from '../reviews/model.js';

const NOW = Date.parse('2026-10-27T12:00:00Z');
const row = (extra) => ({ id: 'o-r-4-ddddddd', repo: 'o/r', number: 4, title: 'Bad', author: 'di', endedAt: '2026-10-27T10:00:00Z', counts: { high: 0, medium: 0, low: 0 }, handle: 'ext-s4', ...extra });
// reply builder: top-level fields (maxAttempts) merge next to reviews
const replyOf = (extra, top) => ({ reviews: [row(extra)], ...top });
const inbox = (extra, top) => m.inboxRows(replyOf(extra, top), NOW)[0];
const failed = { error: 'claude failed', attempts: 2, nextRetryAt: '2026-10-27T13:05:00Z' };

test('failed row with a pending retry says which attempt comes next', () => {
  const r = inbox(failed, { maxAttempts: 5 });
  assert.equal(r.retry, 'retrying at 13:05 (attempt 3 of 5)');
  assert.equal(r.summary, 'claude failed');
  assert.equal(r.tone, 'error');
});

test('the cap is taken from the reply, not hard-coded', () => {
  assert.equal(inbox(failed, { maxAttempts: 7 }).retry, 'retrying at 13:05 (attempt 3 of 7)');
  assert.equal(inbox(failed, { maxAttempts: 3 }).retry, 'retrying at 13:05 (attempt 3 of 3)');
});

test('reply without maxAttempts shows the attempt with no max', () => {
  assert.equal(inbox(failed).retry, 'retrying at 13:05 (attempt 3)');
});

test('maxAttempts that is not a positive integer is ignored, never guessed', () => {
  for (const bad of [null, 0, -1, '5', 2.5, NaN, undefined, true]) {
    assert.equal(inbox(failed, { maxAttempts: bad }).retry, 'retrying at 13:05 (attempt 3)', 'maxAttempts ' + String(bad));
  }
});

test('retry time is zero padded', () => {
  const r = inbox({ error: 'x', attempts: 1, nextRetryAt: '2026-10-27T09:07:00Z' }, { maxAttempts: 5 });
  assert.equal(r.retry, 'retrying at 09:07 (attempt 2 of 5)');
});

test('legacy failed row without attempts counts the retry as attempt 2', () => {
  const r = inbox({ error: 'x', nextRetryAt: '2026-10-27T13:05:00Z' }, { maxAttempts: 5 });
  assert.equal(r.retry, 'retrying at 13:05 (attempt 2 of 5)');
});

test('successful row has no retry text even with stray fields', () => {
  const r = inbox({ attempts: 2, nextRetryAt: '2026-10-27T13:05:00Z' }, { maxAttempts: 5 });
  assert.equal(r.retry, '');
});

test('gave-up row has no retry text and keeps its summary', () => {
  const r = inbox({ error: 'claude exited 1: limit reached; gave up after 5 attempts', attempts: 5 }, { maxAttempts: 5 });
  assert.equal(r.retry, '');
  assert.ok(r.summary.includes('gave up after 5 attempts'));
});

test('unparseable nextRetryAt gives no retry text', () => {
  const r = inbox({ error: 'x', attempts: 1, nextRetryAt: 'soon' }, { maxAttempts: 5 });
  assert.equal(r.retry, '');
});

test('tabRows open rows carry the retry text with the reply cap', () => {
  const reply = replyOf({ state: 'open', ...failed }, { maxAttempts: 7 });
  const open = m.tabRows(reply, NOW).open;
  assert.equal(open.length, 1);
  assert.equal(open[0].retry, 'retrying at 13:05 (attempt 3 of 7)');
});

test('tabRows open rows omit the max when the reply has none', () => {
  const open = m.tabRows(replyOf({ state: 'open', ...failed }), NOW).open;
  assert.equal(open[0].retry, 'retrying at 13:05 (attempt 3)');
});

test('model.js no longer exports MAX_ATTEMPTS', () => {
  assert.ok(!('MAX_ATTEMPTS' in m));
});

test('model.js source has no hand-written attempt cap', () => {
  const src = fs.readFileSync(new URL('../reviews/model.js', import.meta.url), 'utf8');
  assert.ok(!/MAX_ATTEMPTS/.test(src));
});
