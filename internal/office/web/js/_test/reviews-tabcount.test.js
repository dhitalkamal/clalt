// pure model of the reviews panel tab labels with counts, and the open list staying as before. no dom.
import test from 'node:test';
import assert from 'node:assert/strict';
import * as model from '../reviews/model.js';

const { inboxRows, tabRows, showButton } = model;
// tabLabels is read off the namespace so a missing export fails as an assertion, not a link error
const tabLabels = (...a) => {
  assert.equal(typeof model.tabLabels, 'function', 'tabLabels is exported');
  return model.tabLabels(...a);
};

const NOW = Date.parse('2026-10-27T12:00:00Z');
const counts = { high: 0, medium: 0, low: 0 };
const row = (n, state, extra = {}) => ({
  id: 'o-r-' + n + '-abcdef0', repo: 'o/r', number: n, title: 'T' + n, author: 'a' + n,
  endedAt: new Date(NOW - n * 60000).toISOString(), counts, handle: 'ext-s' + n, state, ...extra,
});
const many = (n, state, from = 1) => Array.from({ length: n }, (_, i) => row(from + i, state));

test('tabLabels shows the count of each list', () => {
  const t = tabRows({ reviews: [...many(3, 'open'), ...many(2, 'merged', 10), row(20, 'closed')] }, NOW);
  assert.deepEqual(tabLabels(t), { open: 'Open (3)', history: 'History (3)' });
});

test('tabLabels shows zero as (0)', () => {
  assert.deepEqual(tabLabels({ open: [], history: [] }), { open: 'Open (0)', history: 'History (0)' });
  const onlyOpen = tabRows({ reviews: many(2, 'open') }, NOW);
  assert.deepEqual(tabLabels(onlyOpen), { open: 'Open (2)', history: 'History (0)' });
});

test('tabLabels counts each list on its own', () => {
  const onlyHistory = tabRows({ reviews: many(4, 'merged') }, NOW);
  assert.deepEqual(tabLabels(onlyHistory), { open: 'Open (0)', history: 'History (4)' });
});

test('tabLabels caps counts over 99 like the badge', () => {
  const t = tabRows({ reviews: [...many(100, 'open'), ...many(150, 'closed', 200)] }, NOW);
  assert.deepEqual(tabLabels(t), { open: 'Open (99+)', history: 'History (99+)' });
});

test('tabLabels shows 99 as is and 100 as 99+', () => {
  assert.equal(tabLabels({ open: new Array(99).fill({}), history: new Array(100).fill({}) }).open, 'Open (99)');
  assert.equal(tabLabels({ open: new Array(99).fill({}), history: new Array(100).fill({}) }).history, 'History (99+)');
});

test('tabLabels treats null and missing lists as zero', () => {
  const zero = { open: 'Open (0)', history: 'History (0)' };
  assert.deepEqual(tabLabels(null), zero);
  assert.deepEqual(tabLabels(undefined), zero);
  assert.deepEqual(tabLabels({}), zero);
  assert.deepEqual(tabLabels({ open: [row(1, 'open')] }), { open: 'Open (1)', history: 'History (0)' });
  assert.deepEqual(tabLabels({ history: [row(1, 'merged')] }), { open: 'Open (0)', history: 'History (1)' });
});

// unchanged behavior: an all-open reply behaves as the plain inbox did
test('tabRows open equals inboxRows for an all-open reply, history empty', () => {
  const reply = {
    enabled: true,
    reviews: [
      row(1, 'open', { title: '', author: '', account: 'work' }),
      row(2, 'open', { counts: { high: 2, medium: 1, low: 0 } }),
      row(3, 'open', { error: 'boom' }),
      row(4, 'open', { skipped: 'too big' }),
      row(5, 'open', { counts: { high: 0, medium: 0, low: 3 }, handle: '' }),
    ],
  };
  const t = tabRows(reply, NOW);
  const base = inboxRows(reply, NOW);
  assert.equal(t.open.length, base.length);
  const fields = ['id', 'title', 'ref', 'author', 'handle', 'when', 'at', 'tone', 'summary', 'account', 'tip'];
  assert.deepEqual(t.open.map((r) => fields.map((k) => r[k])), base.map((r) => fields.map((k) => r[k])));
  assert.deepEqual(t.history, []);
});

test('showButton depends only on enabled and count, not on state', () => {
  assert.equal(showButton({ enabled: true, count: 0, reviews: many(2, 'merged') }), true);
  assert.equal(showButton({ enabled: false, count: 0, reviews: many(2, 'open') }), false);
  assert.equal(showButton({ enabled: false, count: 3, reviews: many(3, 'closed') }), true);
  assert.equal(showButton({ enabled: false, count: 3, reviews: many(3, 'open') }), true);
  assert.equal(showButton(null), false);
});

test('the badge counts the reviews still open, not merged or closed ones', () => {
  const reply = { reviews: [{ id: 'a', state: 'open' }, { id: 'b', state: 'merged' }, { id: 'c', state: 'closed' }, { id: 'd' }, { id: 'e', state: 'unknown' }] };
  assert.equal(model.openCount(reply), 3);
  assert.equal(model.openCount(null), 0);
});
