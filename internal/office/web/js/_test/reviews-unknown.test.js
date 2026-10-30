// guard: a row whose state is unknown to the page is never hidden from the open tab. no dom.
import test from 'node:test';
import assert from 'node:assert/strict';
import * as model from '../reviews/model.js';

// both exports are read off the namespace so a missing one fails as an assertion, not a link error
const tabRows = (...a) => {
  assert.equal(typeof model.tabRows, 'function', 'tabRows is exported');
  return model.tabRows(...a);
};
const tabLabels = (...a) => {
  assert.equal(typeof model.tabLabels, 'function', 'tabLabels is exported');
  return model.tabLabels(...a);
};

const NOW = Date.parse('2026-10-27T12:00:00Z');
const counts = { high: 0, medium: 0, low: 0 };
const row = (n, extra = {}) => ({
  id: 'o-r-' + n + '-abcdef0', repo: 'o/r', number: n, title: 'T' + n, author: 'a' + n,
  endedAt: new Date(NOW - n * 60000).toISOString(), counts, handle: 'ext-s' + n, ...extra,
});
// a row with the given state and no other state fields
const withState = (n, state, extra = {}) => row(n, { state, stateCheckedAt: '2026-10-27T11:00:00Z', ...extra });

// every shape of "state the page does not understand"
const ODD_STATES = [
  ['unknown', 'unknown'],
  ['empty string', ''],
  ['null', null],
  ['uppercase OPEN', 'OPEN'],
  ['capitalized Merged', 'Merged'],
  ['draft', 'draft'],
  ['garbage', 'garbage'],
  ['number', 42],
  ['zero', 0],
  ['object', { state: 'merged' }],
  ['array', ['merged']],
  ['boolean true', true],
];

function assertUnknownInOpen(r, label) {
  const t = tabRows({ reviews: [r] }, NOW);
  assert.deepEqual(t.history, [], label + ': not in history');
  assert.equal(t.open.length, 1, label + ': in open');
  assert.equal(t.open[0].id, r.id, label + ': same row');
  assert.equal(t.open[0].state, 'unknown', label + ': state is unknown');
  assert.equal(t.open[0].badge, '', label + ': no badge');
  assert.equal(tabLabels(t).open, 'Open (1)', label + ': counted in open');
  assert.equal(tabLabels(t).history, 'History (0)', label + ': not counted in history');
}

for (const [name, state] of ODD_STATES) {
  test('state ' + name + ' stays in open as unknown', () => {
    assertUnknownInOpen(withState(1, state), name);
  });
}

test('state missing stays in open as unknown', () => {
  const r = row(1, { stateCheckedAt: '2026-10-27T11:00:00Z' });
  assert.equal('state' in r, false);
  assertUnknownInOpen(r, 'missing state');
});

test('state undefined stays in open as unknown', () => {
  assertUnknownInOpen(withState(1, undefined), 'undefined state');
});

test('stateCheckedAt null or missing never hides a row from open', () => {
  assertUnknownInOpen(row(1, { state: 'unknown', stateCheckedAt: null }), 'unknown, checked null');
  assertUnknownInOpen(row(2, { state: 'unknown' }), 'unknown, checked missing');
  assertUnknownInOpen(row(3, {}), 'no state, no checked time');
  assertUnknownInOpen(row(4, { state: null, stateCheckedAt: null }), 'null state, null checked');
});

test('an open row with stateCheckedAt null or missing stays in open', () => {
  for (const extra of [{ stateCheckedAt: null }, {}]) {
    const t = tabRows({ reviews: [row(1, { state: 'open', ...extra })] }, NOW);
    assert.equal(t.open.length, 1);
    assert.equal(t.open[0].state, 'open');
    assert.deepEqual(t.history, []);
  }
});

test('a row with endedAt missing or unparseable still appears in open', () => {
  const bad = [
    ['missing', (n) => { const r = withState(n, 'unknown'); delete r.endedAt; return r; }],
    ['null', (n) => withState(n, 'unknown', { endedAt: null })],
    ['empty', (n) => withState(n, 'unknown', { endedAt: '' })],
    ['garbage', (n) => withState(n, 'unknown', { endedAt: 'not a date' })],
    ['open state, garbage', (n) => withState(n, 'open', { endedAt: 'not a date' })],
    ['no state, missing', (n) => { const r = row(n); delete r.endedAt; return r; }],
  ];
  for (const [name, make] of bad) {
    const r = make(1);
    const t = tabRows({ reviews: [r] }, NOW);
    assert.equal(t.open.length, 1, name + ': in open');
    assert.equal(t.open[0].id, r.id, name + ': same row');
    assert.deepEqual(t.history, [], name + ': not in history');
    assert.equal(t.open[0].badge, '', name + ': no badge');
    assert.equal(tabLabels(t).open, 'Open (1)', name + ': counted');
  }
});

test('unknown rows mixed with merged and closed rows are never pulled into history', () => {
  const reviews = [
    withState(1, 'merged'),
    withState(2, 'closed'),
    ...ODD_STATES.map(([, s], i) => withState(10 + i, s)),
    row(30, { stateCheckedAt: null }),
    row(31, { state: 'unknown', stateCheckedAt: null }),
    withState(32, 'unknown', { endedAt: 'nope' }),
    withState(33, 'merged'),
    withState(34, 'closed'),
    withState(35, 'open'),
  ];
  const t = tabRows({ reviews }, NOW);
  const unknownIds = [
    ...ODD_STATES.map((_, i) => 'o-r-' + (10 + i) + '-abcdef0'),
    'o-r-30-abcdef0', 'o-r-31-abcdef0', 'o-r-32-abcdef0',
  ];
  const openIds = t.open.map((r) => r.id);
  for (const id of unknownIds) assert.ok(openIds.includes(id), id + ' is in open');
  assert.ok(openIds.includes('o-r-35-abcdef0'), 'open row is in open');
  assert.deepEqual(t.history.map((r) => r.id).sort(), ['o-r-1-abcdef0', 'o-r-2-abcdef0', 'o-r-33-abcdef0', 'o-r-34-abcdef0']);
  for (const r of t.history) assert.ok(r.state === 'merged' || r.state === 'closed');
  for (const r of t.open) assert.ok(r.state === 'open' || r.state === 'unknown');
  assert.equal(t.open.length + t.history.length, reviews.length);
  assert.deepEqual(tabLabels(t), { open: 'Open (' + (unknownIds.length + 1) + ')', history: 'History (4)' });
});

test('unknown rows count toward the open label and cap at 99+', () => {
  const reviews = Array.from({ length: 100 }, (_, i) => row(i + 1, { state: i % 2 ? 'garbage' : null }));
  const t = tabRows({ reviews }, NOW);
  assert.equal(t.open.length, 100);
  assert.deepEqual(tabLabels(t), { open: 'Open (99+)', history: 'History (0)' });
});
