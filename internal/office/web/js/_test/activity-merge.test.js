import test from 'node:test';
import assert from 'node:assert/strict';
import { entryKey, mergeNewer, mergeOlder } from '../activity/merge.js';

const mk = (n, kind = 'reply') => ({ t: `2026-10-27T10:00:${String(n).padStart(2, '0')}Z`, kind, text: 'msg ' + n, agent: '' });
// newest first, like the reply
const list = (...ns) => ns.map((n) => mk(n));

test('entryKey differs by time, kind and text', () => {
  assert.notEqual(entryKey(mk(1)), entryKey(mk(2)));
  assert.notEqual(entryKey(mk(1)), entryKey(mk(1, 'prompt')));
  assert.equal(entryKey(mk(1)), entryKey(mk(1)));
});

test('mergeNewer adds only what is newer than the newest known', () => {
  const r = mergeNewer(list(3, 2, 1), list(5, 4, 3, 2, 1));
  assert.deepEqual(r.entries.map((x) => x.text), ['msg 5', 'msg 4', 'msg 3', 'msg 2', 'msg 1']);
  assert.equal(r.added, 2);
});

test('mergeNewer with nothing new changes nothing', () => {
  const known = list(3, 2, 1);
  const r = mergeNewer(known, list(3, 2, 1));
  assert.equal(r.added, 0);
  assert.equal(r.entries, known);
});

test('mergeNewer never duplicates when the batches overlap partly', () => {
  const r = mergeNewer(list(3, 2, 1), list(4, 3, 2));
  assert.deepEqual(r.entries.map((x) => x.text), ['msg 4', 'msg 3', 'msg 2', 'msg 1']);
});

test('mergeNewer keeps two identical entries of the same second', () => {
  const a = { ...mk(2), kind: 'tool', tool: 'Read', text: '' };
  const r = mergeNewer([a, mk(1)], [{ ...a }, { ...a }, mk(1)]);
  assert.equal(r.entries.length, 3);
  assert.equal(r.added, 1);
});

test('mergeNewer with a gap (no overlap) takes the new ones and drops repeats', () => {
  const r = mergeNewer(list(2, 1), list(9, 8, 7));
  assert.deepEqual(r.entries.map((x) => x.text), ['msg 9', 'msg 8', 'msg 7', 'msg 2', 'msg 1']);
});

test('mergeNewer onto an empty list takes everything', () => {
  assert.equal(mergeNewer([], list(2, 1)).entries.length, 2);
});

test('mergeOlder appends older entries and skips ones already known', () => {
  const r = mergeOlder(list(5, 4), list(4, 3, 2));
  assert.deepEqual(r.entries.map((x) => x.text), ['msg 5', 'msg 4', 'msg 3', 'msg 2']);
  assert.equal(r.added, 2);
});
