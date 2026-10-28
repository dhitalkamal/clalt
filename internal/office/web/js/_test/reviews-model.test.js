// pure models of the PR reviews: inbox rows, findings groups, settings section. no dom.
import test from 'node:test';
import assert from 'node:assert/strict';
import { badgeText, showButton, inboxRows, groupFindings, whereText, settingsModel, setEnabled, toggleRepo, addRepo, saveBody, lastCheckedText, runLogRec } from '../reviews/model.js';

const NOW = Date.parse('2026-10-27T12:00:00Z');
const REPLY = {
  enabled: true, running: { repo: 'o/r', number: 9, title: 'Busy one' },
  reviews: [
    { id: 'o-r-1-aaaaaaa', repo: 'o/r', number: 1, title: 'Old', author: 'ann', endedAt: '2026-10-27T09:00:00Z', counts: { high: 0, medium: 0, low: 0 }, handle: 'ext-s1' },
    { id: 'o-r-2-bbbbbbb', repo: 'o/r', number: 2, title: 'New', author: 'bo', endedAt: '2026-10-27T11:30:00Z', counts: { high: 2, medium: 1, low: 0 }, handle: 'ext-s2' },
    { id: 'o-r-3-ccccccc', repo: 'o/r', number: 3, title: 'Big', author: 'cy', endedAt: '2026-10-27T11:00:00Z', counts: { high: 0, medium: 0, low: 0 }, skipped: 'not reviewed: over 1500 changed lines', handle: '' },
    { id: 'o-r-4-ddddddd', repo: 'o/r', number: 4, title: 'Bad', author: 'di', endedAt: '2026-10-27T10:00:00Z', counts: { high: 0, medium: 0, low: 0 }, error: 'claude failed', handle: 'ext-s4' },
  ],
};

test('badgeText caps big counts', () => {
  assert.equal(badgeText(0), '0');
  assert.equal(badgeText(7), '7');
  assert.equal(badgeText(120), '99+');
});

test('showButton is for reviews that are on or that exist', () => {
  assert.equal(showButton(null), false);
  assert.equal(showButton({ enabled: false, count: 0 }), false);
  assert.equal(showButton({ enabled: true, count: 0 }), true);
  assert.equal(showButton({ enabled: false, count: 2 }), true);
});

test('inboxRows is newest first with a ref, a tone and a note', () => {
  const rows = inboxRows(REPLY, NOW);
  assert.deepEqual(rows.map((r) => r.id), ['o-r-2-bbbbbbb', 'o-r-3-ccccccc', 'o-r-4-ddddddd', 'o-r-1-aaaaaaa']);
  assert.equal(rows[0].ref, 'o/r#2');
  assert.equal(rows[0].when, '30m');
  assert.equal(rows[0].tone, 'high');
  assert.equal(rows[0].summary, '2 high, 1 medium');
  assert.equal(rows[1].tone, 'skipped');
  assert.equal(rows[1].summary, 'not reviewed: over 1500 changed lines');
  assert.equal(rows[2].tone, 'error');
  assert.equal(rows[3].tone, 'clean');
  assert.equal(rows[3].summary, 'no findings');
});

test('inboxRows copes with nothing', () => {
  assert.deepEqual(inboxRows(null, NOW), []);
  assert.deepEqual(inboxRows({ reviews: 'x' }, NOW), []);
});

test('inboxRows carry the full title and ref as a tooltip, also when the title is empty', () => {
  const long = 'x'.repeat(200);
  const rows = inboxRows({ reviews: [{ id: 'a', repo: 'o/r', number: 5, title: long, endedAt: '2026-10-27T11:00:00Z' }, { id: 'b', repo: 'o/r', number: 6, title: '', endedAt: '2026-10-27T10:00:00Z' }] }, NOW);
  assert.equal(rows[0].tip, long + ' (o/r#5)');
  assert.equal(rows[1].tip, '(no title) (o/r#6)');
});

test('groupFindings orders high, medium, low and drops empty groups', () => {
  const g = groupFindings([
    { severity: 'low', file: 'c.go', line: 0, title: 'c', detail: '' },
    { severity: 'high', file: 'a.go', line: 3, title: 'a', detail: 'why' },
    { severity: 'weird', file: '', line: 0, title: 'w', detail: '' },
    { severity: 'high', file: 'b.go', line: 8, title: 'b', detail: '' },
  ]);
  assert.deepEqual(g.map((x) => x.severity), ['high', 'low']);
  assert.deepEqual(g[0].items.map((i) => i.where), ['a.go:3', 'b.go:8']);
  assert.equal(g[1].items.length, 2, 'unknown severity counts as low');
  assert.deepEqual(groupFindings(null), []);
});

test('whereText is file:line, file alone, or empty', () => {
  assert.equal(whereText({ file: 'a.go', line: 3 }), 'a.go:3');
  assert.equal(whereText({ file: 'a.go', line: 0 }), 'a.go');
  assert.equal(whereText({ file: '', line: 4 }), '');
});

test('runLogRec is what the activity panel opens, null without a handle', () => {
  assert.deepEqual(runLogRec({ handle: 'ext-s2', title: 'New', repo: 'o/r', number: 2, account: 'home' }), { role: 'review', handle: 'ext-s2', title: 'Review o/r#2', account: 'home', project: 'o/r', state: 'ended' });
  assert.equal(runLogRec({ handle: '' }), null);
});

test('settingsModel merges opted-in repos and suggestions, sorted, once each', () => {
  const m = settingsModel({ enabled: false, repos: ['o/b'], suggestions: ['o/z', 'o/a'], lastChecked: null });
  assert.equal(m.enabled, false);
  assert.deepEqual(m.repos, [{ name: 'o/a', on: false }, { name: 'o/b', on: true }, { name: 'o/z', on: false }]);
  assert.equal(m.lastChecked, 'not checked yet');
});

test('toggle, add, enable and the save body', () => {
  let m = settingsModel({ enabled: false, repos: ['o/b'], suggestions: ['o/a'] });
  m = setEnabled(m, true);
  m = toggleRepo(m, 'o/a', true);
  m = toggleRepo(m, 'o/b', false);
  const added = addRepo(m, ' o/new ');
  assert.equal(added.error, '');
  assert.deepEqual(saveBody(added.model), { enabled: true, repos: ['o/a', 'o/new'] });
  assert.match(addRepo(m, '--web').error, /owner\/name/);
  assert.match(addRepo(m, 'a/b/c').error, /owner\/name/);
  assert.equal(addRepo(m, 'o/a').model.repos.filter((r) => r.name === 'o/a').length, 1);
  assert.equal(m.repos.find((r) => r.name === 'o/b').on, false, 'models are not changed in place');
});

test('lastCheckedText', () => {
  assert.equal(lastCheckedText(null, NOW), 'not checked yet');
  assert.equal(lastCheckedText('2026-10-27T11:55:00Z', NOW), 'checked 5m ago');
  assert.equal(lastCheckedText('2026-10-27T11:59:59Z', NOW), 'checked just now');
});
