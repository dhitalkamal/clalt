// pure model of the reviews panel tabs: open and history lists. no dom.
import test from 'node:test';
import assert from 'node:assert/strict';
import * as model from '../reviews/model.js';

const { inboxRows } = model;
// tabRows is read off the namespace so a missing export fails as an assertion, not a link error
const tabRows = (...a) => {
  assert.equal(typeof model.tabRows, 'function', 'tabRows is exported');
  return model.tabRows(...a);
};

const NOW = Date.parse('2026-10-27T12:00:00Z');
const counts = { high: 0, medium: 0, low: 0 };
const REPLY = {
  enabled: true,
  reviews: [
    { id: 'o-r-1-aaaaaaa', repo: 'o/r', number: 1, title: 'One', author: 'ann', endedAt: '2026-10-27T09:00:00Z', counts, handle: 'ext-s1', state: 'open', stateCheckedAt: '2026-10-27T11:00:00Z' },
    { id: 'o-r-2-bbbbbbb', repo: 'o/r', number: 2, title: 'Two', author: 'bo', endedAt: '2026-10-26T23:30:00Z', counts, handle: 'ext-s2', state: 'merged', stateCheckedAt: '2026-10-27T11:00:00Z' },
    { id: 'o-r-3-ccccccc', repo: 'o/r', number: 3, title: 'Three', author: 'cy', endedAt: '2026-10-27T11:00:00Z', counts, handle: 'ext-s3', state: 'closed', stateCheckedAt: '2026-10-27T11:00:00Z' },
    { id: 'o-r-4-ddddddd', repo: 'o/r', number: 4, title: 'Four', author: 'di', endedAt: '2026-10-27T10:00:00Z', counts, handle: 'ext-s4', state: 'unknown', stateCheckedAt: null },
    { id: 'o-r-5-eeeeeee', repo: 'o/r', number: 5, title: 'Five', author: 'ed', endedAt: '2026-10-27T11:30:00Z', counts, handle: 'ext-s5', state: 'open', stateCheckedAt: '2026-10-27T11:00:00Z' },
    { id: 'o-r-6-fffffff', repo: 'o/r', number: 6, title: 'Six', author: 'flo', endedAt: '2026-10-25T08:00:00Z', counts, handle: 'ext-s6', state: 'merged', stateCheckedAt: '2026-10-27T11:00:00Z' },
  ],
};

test('tabRows open holds open and unknown rows, newest first', () => {
  const t = tabRows(REPLY, NOW);
  assert.deepEqual(t.open.map((r) => r.id), ['o-r-5-eeeeeee', 'o-r-4-ddddddd', 'o-r-1-aaaaaaa']);
  assert.deepEqual(t.open.map((r) => r.state), ['open', 'unknown', 'open']);
  assert.deepEqual(t.open.map((r) => r.badge), ['', '', '']);
});

test('tabRows history holds merged and closed rows interleaved, newest first', () => {
  const t = tabRows(REPLY, NOW);
  assert.deepEqual(t.history.map((r) => r.id), ['o-r-3-ccccccc', 'o-r-2-bbbbbbb', 'o-r-6-fffffff']);
  assert.deepEqual(t.history.map((r) => r.state), ['closed', 'merged', 'merged']);
});

test('tabRows history rows carry a MERGED or CLOSED badge and the utc day', () => {
  const t = tabRows(REPLY, NOW);
  assert.deepEqual(t.history.map((r) => r.badge), ['CLOSED', 'MERGED', 'MERGED']);
  assert.deepEqual(t.history.map((r) => r.date), ['2026-10-27', '2026-10-26', '2026-10-25']);
});

test('tabRows rows keep the inboxRows fields', () => {
  const t = tabRows(REPLY, NOW);
  const base = new Map(inboxRows(REPLY, NOW).map((r) => [r.id, r]));
  for (const r of [...t.open, ...t.history]) {
    const b = base.get(r.id);
    for (const k of Object.keys(b)) assert.deepEqual(r[k], b[k], k + ' of ' + r.id);
  }
  assert.equal(t.history[0].ref, 'o/r#3');
  assert.equal(t.open[0].ref, 'o/r#5');
});

test('tabRows puts every row in exactly one list', () => {
  const t = tabRows(REPLY, NOW);
  const ids = [...t.open, ...t.history].map((r) => r.id);
  assert.equal(ids.length, REPLY.reviews.length);
  assert.equal(new Set(ids).size, REPLY.reviews.length);
  assert.deepEqual([...ids].sort(), REPLY.reviews.map((r) => r.id).sort());
});

test('tabRows copes with nothing', () => {
  assert.deepEqual(tabRows(null, NOW), { open: [], history: [] });
  assert.deepEqual(tabRows({ reviews: 'x' }, NOW), { open: [], history: [] });
});
