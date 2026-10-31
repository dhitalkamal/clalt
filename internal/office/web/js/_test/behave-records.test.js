// how doing reaches the records the scene reads
import test from 'node:test';
import assert from 'node:assert/strict';
import { trackRecord, memberDoing } from '../behave/records.js';
import { currentAct } from '../behave/work.js';

const edit = { kind: 'edit', tool: 'Edit', state: 'running', at: 'a' };

test('trackRecord stores the doing and the act on the record', () => {
  const rec = {};
  trackRecord(rec, edit, 10);
  assert.equal(rec.doing, edit);
  assert.equal(currentAct(rec.track, 10), 'type');
  trackRecord(rec, undefined, 20);
  assert.equal(rec.doing, null);
  assert.equal(currentAct(rec.track, 20), null);
});

test('a team member acts out the work of its first run that has some', () => {
  const runs = [[{}, { doing: null }], [{}, { doing: edit }], [{}, { doing: { kind: 'read' } }]];
  assert.equal(memberDoing(runs), edit);
  assert.equal(memberDoing([]), null);
  assert.equal(memberDoing([[{}, {}]]), null);
});
