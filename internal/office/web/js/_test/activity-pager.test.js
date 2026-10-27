import test from 'node:test';
import assert from 'node:assert/strict';
import { newPager, pagerKey, applyFirst, applyNewer, applyOlder, startOlder, failOlder, canLoadOlder, shouldRefresh, activityPath, matches } from '../activity/pager.js';

const mk = (n) => ({ t: `2026-10-27T10:00:${String(n).padStart(2, '0')}Z`, kind: 'reply', text: 'm' + n, agent: '' });
const reply = (next, ...ns) => ({ handle: 'h1', agent: '', next, entries: ns.map(mk) });

test('activityPath builds the query and omits empty parts', () => {
  assert.equal(activityPath({ handle: 'a b', limit: 100 }), '/api/activity?handle=a+b&limit=100');
  assert.equal(activityPath({ handle: 'h', agent: 'x1', before: 'c/1' }), '/api/activity?handle=h&agent=x1&before=c%2F1');
});

test('pagerKey tells runs of one session apart', () => {
  assert.notEqual(pagerKey({ handle: 'h', agent: '' }), pagerKey({ handle: 'h', agent: 'x' }));
});

test('a fresh pager is not loaded and cannot page', () => {
  const p = newPager({ handle: 'h1', agent: '' });
  assert.equal(p.loaded, false);
  assert.equal(canLoadOlder(p), false);
});

test('applyFirst stores entries and the cursor', () => {
  const p = applyFirst(newPager({ handle: 'h1', agent: '' }), reply('c2', 5, 4, 3));
  assert.equal(p.loaded, true);
  assert.equal(p.next, 'c2');
  assert.equal(canLoadOlder(p), true);
});

test('empty cursor means nothing older', () => {
  const p = applyFirst(newPager({ handle: 'h1', agent: '' }), reply('', 2, 1));
  assert.equal(canLoadOlder(p), false);
});

test('older paging follows the cursor and ends when it is empty', () => {
  let p = applyFirst(newPager({ handle: 'h1', agent: '' }), reply('c2', 5, 4));
  p = startOlder(p);
  assert.equal(canLoadOlder(p), false);
  p = applyOlder(p, reply('', 3, 2));
  assert.equal(p.entries.length, 4);
  assert.equal(p.next, '');
  assert.equal(canLoadOlder(p), false);
});

test('a failed older load can be retried and keeps the cursor', () => {
  let p = applyFirst(newPager({ handle: 'h1', agent: '' }), reply('c2', 5));
  p = failOlder(startOlder(p), 'http 500');
  assert.equal(p.olderError, 'http 500');
  assert.equal(canLoadOlder(p), true);
  assert.equal(p.next, 'c2');
});

test('applyNewer keeps the older cursor and reports additions', () => {
  const p = applyFirst(newPager({ handle: 'h1', agent: '' }), reply('c2', 5, 4));
  const r = applyNewer(p, { newest: 'n7', entries: [mk(7), mk(6)] });
  assert.equal(r.added, 2);
  assert.equal(r.pager.next, 'c2');
  assert.equal(r.pager.entries[0].text, 'm7');
});

test('applyNewer on a log that was empty stays at nothing older', () => {
  const p = applyFirst(newPager({ handle: 'h1', agent: '' }), reply(''));
  const r = applyNewer(p, { entries: [mk(2), mk(1)] });
  assert.equal(r.pager.next, '');
  assert.equal(r.added, 2);
});

test('matches rejects a reply for another run', () => {
  const p = newPager({ handle: 'h1', agent: '' });
  assert.equal(matches(p, { handle: 'h1', agent: '' }), true);
  assert.equal(matches(p, { handle: 'h2', agent: '' }), false);
  assert.equal(matches(p, { handle: 'h1', agent: 'x' }), false);
  assert.equal(matches(p, { handle: 'h1' }), true);
});

test('shouldRefresh needs an open, visible, loaded and idle panel', () => {
  const ok = { open: true, hidden: false, loaded: true, refreshing: false };
  assert.equal(shouldRefresh(ok), true);
  assert.equal(shouldRefresh({ ...ok, open: false }), false);
  assert.equal(shouldRefresh({ ...ok, hidden: true }), false);
  assert.equal(shouldRefresh({ ...ok, refreshing: true }), false);
});
