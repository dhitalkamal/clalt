// roster building for the team office: members always present, first run maps, extra runs are interns
import test from 'node:test';
import assert from 'node:assert/strict';
import { updateMembers, classifySubs, rosterOf, teamIdOf } from '../roster.js';
import { pickLook } from '../looks.js';

const entry = (id, busy = 0, extra = {}) => ({ id, name: id[0].toUpperCase() + id.slice(1), desc: 'does ' + id, source: 'builtin', busy, color: '#4f8edc', ...extra });
const session = (id, subs = [], extra = {}) => ({ id, handle: id, role: 'main', account: 'a', project: 'proj-' + id, state: 'busy', leaving: false, subs, ...extra });
const sub = (session, id, type, extra = {}) => { const s = { id: session.id + '/' + id, role: 'sub', type, session, leaving: false, startedAt: 0, ...extra }; session.subs.push(s); return s; };
const ids = (team) => new Set(team.map((e) => e.id));

test('team ids are the lowercase of the type', () => {
  assert.equal(teamIdOf('Explore'), 'explore');
  assert.equal(teamIdOf('pr-review-toolkit:Code-Reviewer'), 'pr-review-toolkit:code-reviewer');
});

test('every team entry is a member record, idle when nothing runs', () => {
  const m = updateMembers([], [entry('explore'), entry('plan')], []);
  assert.deepEqual(m.map((x) => x.id), ['team:explore', 'team:plan']);
  assert.equal(m[0].role, 'team');
  assert.equal(m[0].team, 'explore');
  assert.equal(m[0].state, 'idle');
  assert.equal(m[0].name, 'Explore');
  assert.equal(m[0].tint, 0x4f8edc);
});

test('a member is busy when the snapshot says busy or a run of its type is live', () => {
  let m = updateMembers([], [entry('explore', 2), entry('plan')], []);
  assert.equal(m[0].state, 'busy');
  const s = session('s1');
  sub(s, 'a', 'Plan');
  m = updateMembers(m, [entry('explore', 0), entry('plan', 0)], [s]);
  assert.equal(m[0].state, 'idle');
  assert.equal(m[1].state, 'busy');
});

test('a leaving run does not keep a member busy', () => {
  const s = session('s1');
  sub(s, 'a', 'Plan', { leaving: true });
  const m = updateMembers([], [entry('plan')], [s]);
  assert.equal(m[0].state, 'idle');
});

test('member records keep identity across snapshots and list the sessions using them', () => {
  const s = session('s1');
  sub(s, 'a', 'Explore');
  const m1 = updateMembers([], [entry('explore', 1)], [s]);
  const m2 = updateMembers(m1, [entry('explore', 1, { desc: 'new' })], [s]);
  assert.equal(m2[0], m1[0]);
  assert.equal(m2[0].desc, 'new');
  assert.deepEqual(m2[0].users, [{ handle: 's1', project: 'proj-s1' }]);
});

test('a member missing from the snapshot leaves, and a team that is gone leaves all', () => {
  const m1 = updateMembers([], [entry('explore'), entry('plan')], []);
  const m2 = updateMembers(m1, [entry('plan')], []);
  assert.equal(m2.find((x) => x.team === 'explore').leaving, true);
  assert.equal(m2.find((x) => x.team === 'plan').leaving, false);
});

test('the first run of a type maps onto its member, later runs are interns owned by their session', () => {
  const s1 = session('s1');
  const s2 = session('s2');
  const a = sub(s1, 'a', 'Explore', { startedAt: 1 });
  const b = sub(s2, 'b', 'explore', { startedAt: 2 });
  const c = sub(s1, 'c', 'Explore', { startedAt: 3 });
  classifySubs([s1, s2], ids([entry('explore')]));
  assert.equal(a.mapped, true);
  assert.equal(a.intern, false);
  assert.equal(b.intern, true);
  assert.equal(b.owner, 's2');
  assert.equal(b.team, 'explore');
  assert.equal(c.intern, true);
  assert.equal(c.owner, 's1');
});

test('classification is sticky: when the mapped run ends the interns stay interns', () => {
  const s = session('s1');
  const a = sub(s, 'a', 'Explore', { startedAt: 1 });
  const b = sub(s, 'b', 'Explore', { startedAt: 2 });
  classifySubs([s], ids([entry('explore')]));
  a.leaving = true;
  classifySubs([s], ids([entry('explore')]));
  assert.equal(b.intern, true);
  const c = sub(s, 'c', 'Explore', { startedAt: 3 });
  classifySubs([s], ids([entry('explore')]));
  assert.equal(c.mapped, true);
});

test('types outside the team stay generic', () => {
  const s = session('s1');
  const a = sub(s, 'a', 'mystery');
  classifySubs([s], ids([entry('explore')]));
  assert.equal(a.team, null);
  assert.equal(a.mapped, false);
  assert.equal(a.intern, false);
});

test('roster: sessions, members, generic subs and interns, mapped runs have no character', () => {
  const s = session('s1');
  sub(s, 'a', 'Explore', { startedAt: 1 });
  sub(s, 'b', 'Explore', { startedAt: 2 });
  sub(s, 'c', 'mystery', { startedAt: 3 });
  const team = [entry('explore', 2)];
  classifySubs([s], ids(team));
  const members = updateMembers([], team, [s]);
  const r = rosterOf([s], members);
  const by = Object.fromEntries(r.map((x) => [x.key, x]));
  assert.deepEqual(Object.keys(by).sort(), ['s1', 's1/b', 's1/c', 'team:explore']);
  assert.deepEqual(by['team:explore'], { key: 'team:explore', kind: 'team', team: 'explore', type: 'explore', state: 'busy' });
  assert.equal(by['s1/b'].kind, 'intern');
  assert.equal(by['s1/b'].team, 'explore');
  assert.equal(by['s1/b'].owner, 's1');
  assert.equal(by['s1/c'].kind, 'sub');
  assert.equal(by.s1.kind, 'main');
});

test('without a team (older server) the roster is the old one', () => {
  const s = session('s1');
  sub(s, 'a', 'Explore');
  classifySubs([s], new Set());
  const r = rosterOf([s], []);
  assert.deepEqual(r.map((x) => [x.key, x.kind]), [['s1', 'main'], ['s1/a', 'sub']]);
});

test('looks come from a hash of the key and do not depend on order or history', () => {
  const list = ['a', 'b', 'c', 'd', 'e', 'f', 'g'];
  const x = pickLook('amber-fox', list);
  pickLook('other', list);
  assert.equal(pickLook('amber-fox', list), x);
  assert.ok(list.includes(x));
  const seen = new Set(['h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'h7', 'h8'].map((k) => pickLook(k, list)));
  assert.ok(seen.size > 2);
});

test('interns carry the colour of their agent and records are found by key', async () => {
  const { findRecord, agentRecords } = await import('../roster.js');
  const s = session('s1');
  const a = sub(s, 'a', 'Explore', { startedAt: 1 });
  const b = sub(s, 'b', 'Explore', { startedAt: 2 });
  classifySubs([s], ids([entry('explore')]));
  const m = updateMembers([], [entry('explore', 2, { color: '#112233' })], [s]);
  assert.equal(b.tint, 0x112233);
  assert.equal(findRecord([s], m, 'team:explore'), m[0]);
  assert.equal(findRecord([s], m, 's1/b'), b);
  assert.deepEqual(agentRecords([s], m).map((r) => r.id), ['team:explore', 's1', 's1/b']);
  assert.ok(!agentRecords([s], m).includes(a));
});
