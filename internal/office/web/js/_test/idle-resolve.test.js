// what an idle agent does: the most specific rule wins
import test from 'node:test';
import assert from 'node:assert/strict';
import { idleChoice, DEFAULT_RULES, normalizeRules, withChoice, roomsOf } from '../idle/resolve.js';

const rules = (o = {}) => ({ ...DEFAULT_RULES, accounts: {}, teams: {}, subtypes: {}, ...o });
const main = { role: 'main', account: 'a1' };
const sub = { role: 'sub', account: 'a1', type: 'Explore' };
const member = { role: 'team', team: 'qa' };
const intern = { role: 'sub', account: 'a1', type: 'qa', intern: true, team: 'qa' };
const rooms = ['r1', 'r2'];

test('defaults keep today: sessions and subs break, team members stay at the desk', () => {
  assert.equal(idleChoice(main, undefined, rooms), 'break');
  assert.equal(idleChoice(sub, null, rooms), 'break');
  assert.equal(idleChoice(member, undefined, rooms), 'desk');
});

test('a session takes its account rule, then the default', () => {
  assert.equal(idleChoice(main, rules({ accounts: { a1: 'hide' } }), rooms), 'hide');
  assert.equal(idleChoice(main, rules({ accounts: { a2: 'hide' }, default: 'desk' }), rooms), 'desk');
});

test('a subagent takes its type rule, then the account, then the default', () => {
  const r = rules({ subtypes: { Explore: 'desk' }, accounts: { a1: 'hide' }, default: 'break' });
  assert.equal(idleChoice(sub, r, rooms), 'desk');
  assert.equal(idleChoice(sub, rules({ accounts: { a1: 'hide' } }), rooms), 'hide');
  assert.equal(idleChoice(sub, rules({ default: 'hide' }), rooms), 'hide');
});

test('a team member takes its own rule, else the team default, never the account', () => {
  assert.equal(idleChoice(member, rules({ teams: { qa: 'break' } }), rooms), 'break');
  assert.equal(idleChoice(member, rules({ teamDefault: 'hide', default: 'break' }), rooms), 'hide');
  assert.equal(idleChoice({ ...member, account: 'a1' }, rules({ accounts: { a1: 'hide' } }), rooms), 'desk');
});

test('an intern takes the team rule, then type, then account, then default', () => {
  assert.equal(idleChoice(intern, rules({ teams: { qa: 'hide' }, subtypes: { qa: 'desk' } }), rooms), 'hide');
  assert.equal(idleChoice(intern, rules({ subtypes: { qa: 'desk' }, accounts: { a1: 'hide' } }), rooms), 'desk');
  assert.equal(idleChoice(intern, rules({ accounts: { a1: 'hide' } }), rooms), 'hide');
  assert.equal(idleChoice(intern, rules({ default: 'desk' }), rooms), 'desk');
});

test('a room choice becomes {room} when the room exists', () => {
  assert.deepEqual(idleChoice(main, rules({ accounts: { a1: 'room:r2' } }), rooms), { room: 'r2' });
});

test('a room that is gone, or any room in a garden, falls back to a break', () => {
  assert.equal(idleChoice(main, rules({ accounts: { a1: 'room:zzz' } }), rooms), 'break');
  assert.equal(idleChoice(main, rules({ accounts: { a1: 'room:r1' } }), []), 'break');
  assert.equal(idleChoice(main, rules({ default: 'room:r1' }), undefined), 'break');
});

test('an unknown value falls back to a break', () => {
  assert.equal(idleChoice(main, rules({ default: 'dance' }), rooms), 'break');
});

test('normalizeRules fills holes and drops junk', () => {
  assert.deepEqual(normalizeRules(null), DEFAULT_RULES);
  assert.deepEqual(normalizeRules({ default: 'hide', accounts: { a: 'desk' } }),
    { default: 'hide', teamDefault: 'desk', accounts: { a: 'desk' }, teams: {}, subtypes: {} });
  assert.deepEqual(normalizeRules({ accounts: 'x', teams: [], subtypes: { t: 5 } }).subtypes, {});
});

test('withChoice sets and clears one rule without touching the input', () => {
  const r = rules();
  const a = withChoice(r, 'accounts', 'a1', 'hide');
  assert.equal(a.accounts.a1, 'hide');
  assert.deepEqual(r.accounts, {});
  assert.deepEqual(withChoice(a, 'accounts', 'a1', '').accounts, {});
  assert.equal(withChoice(r, 'default', null, 'desk').default, 'desk');
  assert.equal(withChoice(r, 'teamDefault', null, 'hide').teamDefault, 'hide');
});

test('roomsOf lists the rooms of a hall without lobby and cabins, none for a garden', () => {
  const hall = { v: 1, rooms: [{ id: 'l', type: 'lobby' }, { id: 'c', type: 'cabins' }, { id: 'dev', type: 'dev', label: 'Dev' }, { id: 'cafe', type: 'cafe' }] };
  assert.deepEqual(roomsOf(hall), [{ id: 'dev', label: 'Dev' }, { id: 'cafe', label: 'cafe' }]);
  assert.deepEqual(roomsOf({ v: 2, rooms: hall.rooms }), []);
  assert.deepEqual(roomsOf(null), []);
});
