import test from 'node:test';
import assert from 'node:assert/strict';
import { activityTarget, agentOfSub } from '../activity/target.js';

const session = { id: 'h1', role: 'main', handle: 'h1', project: 'proj', account: 'work', state: 'busy', subs: [], leaving: false };
const sub = (id, extra = {}) => ({ id: 'h1/' + id, role: 'sub', type: 'Explore', session, startedAt: 1, leaving: false, mapped: false, intern: false, team: null, task: 't', ...extra });

test('agentOfSub strips the session handle', () => {
  assert.equal(agentOfSub({ id: 'h1/abc', session }), 'abc');
  assert.equal(agentOfSub({ id: 'weird', session }), '');
});

test('a session opens its main conversation', () => {
  assert.deepEqual(activityTarget(session, [session]), { handle: 'h1', agent: '', kind: 'session' });
});

test('an intern or plain subagent opens its own run', () => {
  assert.deepEqual(activityTarget(sub('x1', { intern: true, team: 'reviewer' }), [session]), { handle: 'h1', agent: 'x1', kind: 'run' });
  assert.deepEqual(activityTarget(sub('x2'), [session]), { handle: 'h1', agent: 'x2', kind: 'run' });
});

test('a team member opens its mapped run', () => {
  const m = sub('m1', { mapped: true, team: 'reviewer' });
  const other = { ...session, id: 'h2', handle: 'h2', subs: [] };
  session.subs = [sub('x9', { intern: true, team: 'reviewer', startedAt: 9 }), m];
  const member = { id: 'team:reviewer', role: 'team', team: 'reviewer', state: 'busy' };
  assert.deepEqual(activityTarget(member, [other, session]), { handle: 'h1', agent: 'm1', kind: 'run' });
});

test('a busy member with only interns opens the newest intern', () => {
  session.subs = [sub('x1', { intern: true, team: 'qa', startedAt: 1 }), sub('x2', { intern: true, team: 'qa', startedAt: 5 })];
  const member = { id: 'team:qa', role: 'team', team: 'qa', state: 'busy' };
  assert.equal(activityTarget(member, [session]).agent, 'x2');
});

test('an idle member has no run to show', () => {
  session.subs = [];
  assert.equal(activityTarget({ id: 'team:qa', role: 'team', team: 'qa', state: 'idle' }, [session]), null);
});

test('leaving runs are skipped', () => {
  session.subs = [sub('x1', { mapped: true, team: 'qa', leaving: true })];
  assert.equal(activityTarget({ id: 'team:qa', role: 'team', team: 'qa', state: 'busy' }, [session]), null);
});

import { describeTarget } from '../activity/target.js';

test('describeTarget gives the header fields for each kind of character', () => {
  const accounts = new Map([['work', { label: 'Work' }]]);
  const main = describeTarget(session, accounts, [session]);
  assert.equal(main.account, 'Work');
  assert.equal(main.project, 'proj');
  assert.equal(main.state, 'busy');
  const intern = describeTarget(sub('x1', { intern: true, account: 'work' }), accounts, [session]);
  assert.equal(intern.title, 'Explore (intern)');
  assert.equal(intern.project, 'proj');
  const member = describeTarget({ role: 'team', name: 'Reviewer', team: 'reviewer', state: 'idle' }, accounts, [session]);
  assert.equal(member.stateText, 'resting');
});
