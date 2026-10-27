import test from 'node:test';
import assert from 'node:assert/strict';
import { groupEntries } from '../activity/group.js';

const e = (kind, agent, extra = {}) => ({ t: '2026-10-27T10:00:00Z', kind, agent, text: '', ...extra });

test('entries without an agent stay at the top level', () => {
  const items = groupEntries([e('prompt', ''), e('reply', '')]);
  assert.equal(items.length, 2);
  assert.ok(items.every((i) => i.entry));
});

test('start, inner entries and end form one group at the start position', () => {
  const items = groupEntries([e('prompt', ''), e('subagent-start', 'a1', { text: 'explore' }), e('tool', 'a1', { tool: 'Read' }), e('subagent-end', 'a1', { ok: true }), e('reply', '')]);
  assert.equal(items.length, 3);
  const g = items[1].group;
  assert.equal(g.agent, 'a1');
  assert.equal(g.state, 'done');
  assert.equal(g.entries.length, 1);
  assert.equal(g.start.text, 'explore');
});

test('a start without an end is running', () => {
  const g = groupEntries([e('subagent-start', 'a1')])[0].group;
  assert.equal(g.state, 'running');
});

test('inner entries whose start is on an older page still group', () => {
  const items = groupEntries([e('tool', 'a2', { tool: 'Bash' }), e('subagent-end', 'a2')]);
  assert.equal(items.length, 1);
  assert.equal(items[0].group.start, null);
  assert.equal(items[0].group.state, 'done');
});

test('two interleaved runs keep separate groups', () => {
  const items = groupEntries([e('subagent-start', 'a'), e('subagent-start', 'b'), e('tool', 'b'), e('tool', 'a')]);
  assert.equal(items.length, 2);
  assert.equal(items[0].group.entries.length, 1);
  assert.equal(items[1].group.entries.length, 1);
});

test('flat mode drops markers and keeps every entry at the top level', () => {
  const items = groupEntries([e('subagent-start', 'a'), e('tool', 'a'), e('subagent-end', 'a')], { flat: true });
  assert.equal(items.length, 1);
  assert.equal(items[0].entry.kind, 'tool');
});
