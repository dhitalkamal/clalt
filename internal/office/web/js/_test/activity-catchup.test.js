import test from 'node:test';
import assert from 'node:assert/strict';
import { fetchNewer } from '../activity/catchup.js';
import { activityPath, newPager, applyFirst, applyNewer } from '../activity/pager.js';
import { errorText } from '../activity/api.js';

const mk = (n) => ({ t: `2026-10-27T10:00:${String(n).padStart(2, '0')}Z`, kind: 'reply', text: 'm' + n, agent: '' });
const target = { handle: 'h1', agent: '' };

// a fake route over entries 1..top: after=<n> returns entries above n newest first, paged by limit with cursors
function fake(top, log) {
  return async (params) => {
    log.push(params);
    const after = params.after ? Number(params.after) : 0;
    const upTo = params.before ? Number(params.before) : top;
    const all = [];
    for (let n = upTo; n > after; n--) all.push(mk(n));
    const page = all.slice(0, params.limit);
    const next = all.length > params.limit ? String(upTo - params.limit) : '';
    return { handle: 'h1', agent: '', next, newest: String(top), entries: page };
  };
}

test('activityPath carries after', () => {
  assert.equal(activityPath({ handle: 'h', after: 'n9', limit: 50 }), '/api/activity?handle=h&after=n9&limit=50');
});

test('a normal tick sends after=newest and returns only the new entries', async () => {
  const log = [];
  const r = await fetchNewer(fake(7, log), target, '5', { limit: 100 });
  assert.equal(log.length, 1);
  assert.equal(log[0].after, '5');
  assert.deepEqual(r.entries.map((e) => e.text), ['m7', 'm6']);
  assert.equal(r.newest, '7');
  assert.equal(r.complete, true);
});

test('a quiet tick returns nothing and the same newest', async () => {
  const log = [];
  const r = await fetchNewer(fake(5, log), target, '5', { limit: 100 });
  assert.equal(r.entries.length, 0);
  assert.equal(r.newest, '5');
  assert.equal(log.length, 1);
});

test('a burst bigger than a page follows next with before and the same after', async () => {
  const log = [];
  const r = await fetchNewer(fake(25, log), target, '5', { limit: 8 });
  assert.equal(r.entries.length, 20);
  assert.equal(r.entries[0].text, 'm25');
  assert.equal(r.entries[19].text, 'm6');
  assert.equal(new Set(r.entries.map((e) => e.text)).size, 20);
  assert.equal(log.length, 3);
  assert.equal(log[1].before, '17');
  assert.equal(log[1].after, '5');
  assert.equal(r.newest, '25');
});

test('a runaway burst stops at maxPages and says it is incomplete', async () => {
  const r = await fetchNewer(fake(1000, []), target, '1', { limit: 10, maxPages: 3 });
  assert.equal(r.entries.length, 30);
  assert.equal(r.complete, false);
});

test('an empty log with no newest keeps the old after', async () => {
  const r = await fetchNewer(async () => ({ handle: 'h1', agent: '', next: '', newest: '', entries: [] }), target, '', { limit: 10 });
  assert.equal(r.newest, '');
});

test('applyFirst remembers newest and applyNewer moves it forward', () => {
  let p = applyFirst(newPager(target), { handle: 'h1', agent: '', next: '', newest: '5', entries: [mk(5)] });
  assert.equal(p.newest, '5');
  const r = applyNewer(p, { entries: [mk(7), mk(6)], newest: '7' });
  assert.equal(r.pager.newest, '7');
  assert.equal(r.added, 2);
  p = applyNewer(r.pager, { entries: [], newest: '' }).pager;
  assert.equal(p.newest, '7');
});

test('errorText says what a missing transcript means', () => {
  const nf = Object.assign(new Error('http 404'), { status: 404 });
  assert.equal(errorText(nf), 'no transcript for this session yet');
  const bad = Object.assign(new Error('http 400'), { status: 400 });
  assert.match(errorText(bad), /refused/);
  assert.match(errorText(new Error('http 500')), /http 500/);
});
