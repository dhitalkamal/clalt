// a review's run log opens in the activity panel through a record with role review.
import test from 'node:test';
import assert from 'node:assert/strict';
import { activityTarget, describeTarget } from '../activity/target.js';

const rec = { role: 'review', handle: 'ext-s2', title: 'Review o/r#2', account: 'home', project: 'o/r', state: 'ended' };

test('a review record opens the session log of its handle', () => {
  assert.deepEqual(activityTarget(rec, []), { handle: 'ext-s2', agent: '', kind: 'session' });
});

test('a review record is described by its own title, account and repo', () => {
  const d = describeTarget(rec, new Map([['home', { label: 'HOME' }]]), []);
  assert.equal(d.title, 'Review o/r#2');
  assert.equal(d.account, 'HOME');
  assert.equal(d.project, 'o/r');
  assert.equal(d.state, 'ended');
});
