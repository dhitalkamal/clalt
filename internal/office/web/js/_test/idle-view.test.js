// the words and lists on the settings page and the quick menu
import test from 'node:test';
import assert from 'node:assert/strict';
import { summaryText, optionsFor, targetOf, subtypesOf, customCount, choiceName, scopeNote } from '../idle/view.js';
import { DEFAULT_RULES } from '../idle/resolve.js';

const rooms = [{ id: 'dev', label: 'Dev room' }];

test('the summary names both defaults and counts custom choices', () => {
  assert.equal(summaryText(DEFAULT_RULES, rooms), 'everyone: take a break, team members: stay at desk, no custom choices');
  const r = { ...DEFAULT_RULES, accounts: { a: 'hide' }, teams: { t: 'desk' } };
  assert.equal(summaryText(r, rooms), 'everyone: take a break, team members: stay at desk, 2 custom choices');
  assert.match(summaryText({ ...DEFAULT_RULES, subtypes: { x: 'desk' } }, rooms), /, 1 custom choice$/);
});

test('choiceName reads a room by its label, a missing room as a break', () => {
  assert.equal(choiceName('room:dev', rooms), 'rest in Dev room');
  assert.equal(choiceName('room:gone', rooms), 'take a break');
  assert.equal(choiceName('hide', rooms), 'hide until it works again');
});

test('options: same-as first when given, then the three choices, then a room each', () => {
  const o = optionsFor(rooms, 'everyone');
  assert.deepEqual(o.map((x) => x.value), ['', 'desk', 'break', 'hide', 'room:dev']);
  assert.equal(o[0].text, 'Same as everyone');
  assert.equal(o[1].text, 'Stay at desk');
  assert.equal(o[2].text, 'Take a break (cafe, play area)');
  assert.equal(o[3].text, 'Hide until it works again');
  assert.equal(o[4].text, 'Rest in Dev room');
  assert.deepEqual(optionsFor([], null).map((x) => x.value), ['desk', 'break', 'hide']);
});

test('options keep a saved room that is gone, labelled as taking a break', () => {
  const o = optionsFor(rooms, null, 'room:gone');
  assert.equal(o[o.length - 1].value, 'room:gone');
  assert.match(o[o.length - 1].text, /missing/);
});

test('the quick menu edits the rule that matches the agent', () => {
  const label = (id) => 'Label ' + id;
  assert.deepEqual(targetOf({ role: 'team', team: 'qa' }, label), { scope: 'teams', id: 'qa', same: 'all team members', saved: 'saved for this team member', hint: 'applies to all qa team members' });
  assert.deepEqual(targetOf({ role: 'sub', intern: true, team: 'qa', type: 'qa' }, label).scope, 'teams');
  assert.deepEqual(targetOf({ role: 'sub', type: 'Explore', account: 'a' }, label), { scope: 'subtypes', id: 'Explore', same: 'everyone', saved: 'saved for every Explore subagent', hint: 'applies to every Explore subagent' });
  assert.deepEqual(targetOf({ role: 'main', account: 'a' }, label), { scope: 'accounts', id: 'a', same: 'everyone', saved: 'saved for every session of Label a', hint: 'applies to every session of Label a' });
  assert.equal(targetOf({ role: 'review' }, label), null);
});

test('subagent types come from the live runs and the saved rules, sorted, interns left out', () => {
  const sessions = [{ subs: [{ type: 'Plan' }, { type: 'Explore' }, { type: 'qa', intern: true }] }, { subs: [{ type: 'Plan' }] }];
  assert.deepEqual(subtypesOf(sessions, { Zed: 'desk', Plan: 'hide' }), ['Explore', 'Plan', 'Zed']);
  assert.deepEqual(subtypesOf([], {}), []);
});

test('customCount adds up the three maps', () => {
  assert.equal(customCount(DEFAULT_RULES), 0);
  assert.equal(customCount({ ...DEFAULT_RULES, accounts: { a: 'hide', b: 'desk' }, teams: { t: 'desk' } }), 3);
});

test('the scope line is a hint until something was saved in this panel', () => {
  const t = targetOf({ role: 'team', team: 'qa' }, (id) => id);
  assert.equal(scopeNote(t, false), 'applies to all qa team members');
  assert.equal(scopeNote(t, true), 'saved for this team member');
});
