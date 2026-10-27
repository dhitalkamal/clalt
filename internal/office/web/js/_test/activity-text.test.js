import test from 'node:test';
import assert from 'node:assert/strict';
import { truncateText, snippet, formatDuration, clock, fileSummary } from '../activity/text.js';

test('truncateText keeps short text whole and newlines intact', () => {
  const r = truncateText('a\nb\n\nc', { maxChars: 100, maxLines: 10 });
  assert.deepEqual(r, { text: 'a\nb\n\nc', cut: false, hidden: 0 });
});

test('truncateText cuts by lines then by chars and reports what is hidden', () => {
  const byLines = truncateText('1\n2\n3\n4', { maxChars: 100, maxLines: 2 });
  assert.equal(byLines.text, '1\n2');
  assert.equal(byLines.cut, true);
  assert.equal(byLines.hidden, 4);
  const byChars = truncateText('abcdefghij', { maxChars: 4, maxLines: 10 });
  assert.equal(byChars.text, 'abcd');
  assert.equal(byChars.hidden, 6);
});

test('truncateText tolerates non-strings', () => {
  assert.equal(truncateText(undefined).text, '');
  assert.equal(truncateText(42).text, '');
});

test('snippet is one line with an ascii ellipsis', () => {
  assert.equal(snippet('first\nsecond', 50), 'first');
  assert.equal(snippet('abcdefghij', 6), 'abc...');
  assert.equal(snippet('  \n  x', 50), 'x');
  assert.equal(snippet('', 5), '');
});

test('formatDuration', () => {
  assert.equal(formatDuration(0), '');
  assert.equal(formatDuration(-5), '');
  assert.equal(formatDuration(NaN), '');
  assert.equal(formatDuration(350), '350ms');
  assert.equal(formatDuration(1200), '1.2s');
  assert.equal(formatDuration(12000), '12s');
  assert.equal(formatDuration(125000), '2m 05s');
  assert.equal(formatDuration(3720000), '1h 02m');
});

test('clock shows the time today and the date on other days', () => {
  const now = new Date(2026, 9, 27, 15, 0, 0);
  assert.equal(clock(new Date(2026, 9, 27, 14, 3, 9).toISOString(), now), '14:03:09');
  assert.equal(clock(new Date(2026, 9, 26, 9, 5, 0).toISOString(), now), 'Oct 26 09:05');
  assert.equal(clock('nonsense', now), '');
});

test('fileSummary shortens paths and counts the rest', () => {
  assert.deepEqual(fileSummary([]), { text: '', title: '' });
  assert.deepEqual(fileSummary(['a.go']), { text: 'a.go', title: 'a.go' });
  const r = fileSummary(['/Users/x/proj/internal/office/web/js/app.js', 'b.go', 'c.go']);
  assert.equal(r.text, '.../web/js/app.js +2');
  assert.equal(r.title, '/Users/x/proj/internal/office/web/js/app.js\nb.go\nc.go');
});
