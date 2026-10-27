import test from 'node:test';
import assert from 'node:assert/strict';
import { summaryModel } from '../summary/model.js';
import { summaryText } from '../summary/copytext.js';
import { sample, empty } from './summary-fixture.js';

test('model has headline facts and sections per account and project', () => {
  const m = summaryModel(sample());
  assert.equal(m.empty, false);
  assert.deepEqual(m.headline, [['Sessions', '2'], ['Active', '1h 05m'], ['Tokens', '1.7k'], ['Cost', '$0.01']]);
  assert.equal(m.accounts.length, 2);
  const w = m.accounts[1];
  assert.equal(w.title, 'WORK');
  assert.deepEqual(w.facts, [['Sessions', '1'], ['Active', '5m'], ['Tokens', '540'], ['Cost', '$0.00']]);
  const p = w.projects[0];
  assert.equal(p.title, 'alpha');
  assert.equal(p.subagents, 'reviewer x2, explorer x1');
  assert.equal(p.tools, 'Edit x2, Bash x1, Read x1');
  assert.equal(p.files, 'src/a.go x2, README.md');
  assert.deepEqual(p.failures, ['Bash: boom: build failed']);
});

test('model of an empty day says so and has no sections', () => {
  const m = summaryModel(empty());
  assert.equal(m.empty, true);
  assert.deepEqual(m.accounts, []);
});

test('model tolerates missing lists', () => {
  const m = summaryModel({ day: '2026-10-27', totals: {}, accounts: [{ id: 'a', label: 'A', projects: [{ name: 'p' }] }] });
  assert.equal(m.accounts[0].projects[0].tools, '');
  assert.deepEqual(m.accounts[0].projects[0].failures, []);
});

test('truncated is carried to the model', () => {
  assert.equal(summaryModel({ ...sample(), truncated: true }).truncated, true);
});

test('copy as text is plain ascii lines', () => {
  const text = summaryText(sample());
  assert.equal(text, [
    'clalt summary for 2026-10-27',
    'total: 2 sessions, 1h 05m active, 1.7k tokens, $0.01',
    '',
    'PERSONAL: 1 session, 0m active, 1.5k tokens, $0.01',
    '  beta: 1 session, 0m active, 1.5k tokens, $0.01',
    '    tools: Write x1',
    '    files: notes.txt',
    '',
    'WORK: 1 session, 5m active, 540 tokens, $0.00',
    '  alpha: 1 session, 5m active, 540 tokens, $0.00',
    '    subagents: reviewer x2, explorer x1',
    '    tools: Edit x2, Bash x1, Read x1',
    '    files: src/a.go x2, README.md',
    '    failures: 1',
    '      Bash: boom: build failed',
    '',
  ].join('\n'));
  assert.match(text, /^[\x09\x0a\x20-\x7e]*$/);
});

test('copy as text for an empty day', () => {
  assert.equal(summaryText(empty()), 'clalt summary for 2026-10-27\nnothing happened on this day.\n');
});

test('copy as text marks a cut-short read', () => {
  assert.match(summaryText({ ...sample(), truncated: true }), /some transcripts were not read/);
});

test('copy as text turns non ascii and control characters into plain ones', () => {
  const s = sample();
  s.accounts[1].projects[0].name = 'caf\u00e9\u2014x\u0007';
  const t = summaryText(s);
  assert.match(t, /^[\x09\x0a\x20-\x7e]*$/);
  assert.match(t, /caf\?\?x/);
});
