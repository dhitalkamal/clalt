import test from 'node:test';
import assert from 'node:assert/strict';
import { buildRows } from '../activity/rows.js';

const e = (n, kind, extra = {}) => ({ t: `2026-10-27T10:00:${String(n).padStart(2, '0')}Z`, kind, text: '', tool: '', files: [], durationMs: 0, ok: true, agent: '', ...extra });
const now = new Date(2026, 9, 27, 23, 0, 0);

test('rows come out oldest first although the reply is newest first', () => {
  const rows = buildRows([e(3, 'reply', { text: 'c' }), e(2, 'tool', { tool: 'Read' }), e(1, 'prompt', { text: 'a' })], { now });
  assert.deepEqual(rows.map((r) => r.type), ['text', 'tool', 'text']);
  assert.equal(rows[0].role, 'prompt');
  assert.equal(rows[2].role, 'reply');
});

test('text rows keep newlines and flag long text as cut', () => {
  const long = 'x'.repeat(5000);
  const rows = buildRows([e(2, 'reply', { text: long }), e(1, 'prompt', { text: 'a\nb' })], { now });
  assert.equal(rows[0].text, 'a\nb');
  assert.equal(rows[0].cut, false);
  assert.equal(rows[1].cut, true);
  assert.equal(rows[1].full.length, 5000);
  assert.ok(rows[1].text.length < 5000);
});

test('tool rows carry tool, files, duration and status', () => {
  const [r] = buildRows([e(1, 'tool', { tool: 'Edit', files: ['a.go', 'b.go'], durationMs: 1500, ok: false })], { now });
  assert.equal(r.tool, 'Edit');
  assert.equal(r.files.text, 'a.go +1');
  assert.equal(r.duration, '1.5s');
  assert.equal(r.status, 'failed');
});

test('result rows show a one-line snippet and the full text on demand', () => {
  const [r] = buildRows([e(1, 'result', { text: 'line one\nline two', ok: true })], { now });
  assert.equal(r.type, 'result');
  assert.equal(r.snippet, 'line one');
  assert.equal(r.status, 'ok');
  assert.equal(r.cut, true);
});

test('subagent markers become one group row with its inner rows', () => {
  const rows = buildRows([e(4, 'subagent-end', { agent: 'a1', durationMs: 3000 }), e(3, 'tool', { agent: 'a1', tool: 'Read' }), e(2, 'subagent-start', { agent: 'a1', text: 'look around' }), e(1, 'prompt', { text: 'go' })], { now });
  assert.equal(rows.length, 2);
  const g = rows[1];
  assert.equal(g.type, 'group');
  assert.equal(g.title, 'look around');
  assert.equal(g.state, 'done');
  assert.equal(g.duration, '3.0s');
  assert.equal(g.children.length, 1);
  assert.equal(g.children[0].tool, 'Read');
});

test('a running group says so and a group without text gets a fallback title', () => {
  const [g] = buildRows([e(1, 'subagent-start', { agent: 'abcdef123456' })], { now });
  assert.equal(g.state, 'running');
  assert.equal(g.title, 'subagent abcdef12');
});

test('row ids are unique even for identical entries', () => {
  const rows = buildRows([e(1, 'tool', { tool: 'Read' }), e(1, 'tool', { tool: 'Read' })], { now });
  assert.notEqual(rows[0].id, rows[1].id);
});

test('row ids stay the same when newer entries are added', () => {
  const a = buildRows([e(1, 'tool', { tool: 'Read' })], { now });
  const b = buildRows([e(2, 'reply', { text: 'x' }), e(1, 'tool', { tool: 'Read' })], { now });
  assert.equal(a[0].id, b[0].id);
});

test('unknown kinds are shown as notes, never dropped or thrown on', () => {
  const [r] = buildRows([e(1, 'weird', { text: 'hello' })], { now });
  assert.equal(r.role, 'note');
  assert.equal(r.text, 'hello');
});

test('flat mode returns the run without groups', () => {
  const rows = buildRows([e(2, 'tool', { agent: 'a1', tool: 'Bash' }), e(1, 'subagent-start', { agent: 'a1' })], { now, flat: true });
  assert.deepEqual(rows.map((r) => r.type), ['tool']);
});

test('a tool detail that only repeats the file is dropped', () => {
  const [r] = buildRows([e(1, 'tool', { tool: 'Read', files: ['src/parser.go'], text: 'src/parser.go' })], { now });
  assert.equal(r.detail, '');
  const [q] = buildRows([e(1, 'tool', { tool: 'Bash', files: ['src/parser.go'], text: 'go test ./...' })], { now });
  assert.equal(q.detail, 'go test ./...');
});
