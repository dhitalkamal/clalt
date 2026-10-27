import test from 'node:test';
import assert from 'node:assert/strict';
import { actionView, OUTSIDE } from '../actions/availability.js';
import { forecastView } from '../actions/forecast.js';
import { rankAccounts, planMoveAll } from '../actions/plan.js';
import { confirmStop, confirmMoveAll, moveRequest, sendAction } from '../actions/text.js';

const NOW = Date.parse('2026-10-27T12:00:00Z');
const inMin = (m) => new Date(NOW + m * 60000).toISOString();

test('actionView enables what the session allows', () => {
  const v = actionView({ move: true, focus: true, stop: true, why: '' });
  assert.deepEqual(v.move, { enabled: true, why: '' });
  assert.equal(v.focus.enabled && v.stop.enabled, true);
});

test('actionView puts the reason on every disabled action', () => {
  const v = actionView({ move: false, focus: false, stop: false, why: OUTSIDE });
  for (const k of ['move', 'focus', 'stop']) assert.deepEqual(v[k], { enabled: false, why: OUTSIDE });
  assert.equal(actionView({ move: true, focus: false, stop: true, why: 'no front' }).focus.why, 'no front');
});

test('actionView copes with a missing or reasonless answer', () => {
  assert.equal(actionView(undefined).stop.enabled, false);
  assert.match(actionView(undefined).stop.why, /control/);
  assert.match(actionView({ move: false, focus: true, stop: true, why: '' }).move.why, /not available/);
});

test('forecastView shows nothing until an hour is left', () => {
  assert.equal(forecastView({ fiveHourAt: null, pacePctPerHour: 0 }, NOW), null);
  assert.equal(forecastView(undefined, NOW), null);
  assert.equal(forecastView({ fiveHourAt: inMin(60), pacePctPerHour: 10 }, NOW), null);
  assert.equal(forecastView({ fiveHourAt: inMin(90), pacePctPerHour: 10 }, NOW), null);
});

test('forecastView is amber under 60 minutes and red under 20', () => {
  assert.deepEqual(forecastView({ fiveHourAt: inMin(25) }, NOW), { text: 'full in ~25m', level: 'amber', minutes: 25 });
  assert.equal(forecastView({ fiveHourAt: inMin(59) }, NOW).level, 'amber');
  assert.equal(forecastView({ fiveHourAt: inMin(20) }, NOW).level, 'amber');
  assert.deepEqual(forecastView({ fiveHourAt: inMin(15) }, NOW), { text: 'full in ~15m', level: 'red', minutes: 15 });
  assert.equal(forecastView({ fiveHourAt: inMin(19) }, NOW).level, 'red');
});

test('forecastView rounds up partial minutes and says full now when it is past', () => {
  assert.equal(forecastView({ fiveHourAt: new Date(NOW + 14 * 60000 + 5000).toISOString() }, NOW).text, 'full in ~15m');
  assert.deepEqual(forecastView({ fiveHourAt: inMin(0) }, NOW), { text: 'full now', level: 'red', minutes: 0 });
  assert.equal(forecastView({ fiveHourAt: inMin(-3) }, NOW).text, 'full now');
  assert.equal(forecastView({ fiveHourAt: 'garbage' }, NOW), null);
});

const accounts = new Map([
  ['work', { label: 'WORK', limits: { fiveHour: { usedPct: 80 }, readAt: 'x' }, forecast: { fiveHourAt: inMin(15) } }],
  ['personal', { label: 'PERSONAL', limits: { fiveHour: { usedPct: 12.4 }, readAt: 'x' }, forecast: { fiveHourAt: null } }],
  ['side', { label: 'SIDE', limits: { fiveHour: { usedPct: 40 }, readAt: 'x' } }],
  ['fresh', { label: 'FRESH', limits: { fiveHour: null, readAt: 'x' } }],
  ['unknown', { label: 'UNKNOWN', limits: { fiveHour: null, readAt: null } }],
]);

test('rankAccounts puts the most room first and leaves the account itself out', () => {
  const r = rankAccounts(accounts, 'work');
  assert.deepEqual(r.map((a) => a.id), ['fresh', 'personal', 'side', 'unknown']);
  assert.equal(r.find((a) => a.id === 'personal').pct, 12);
  assert.equal(r.find((a) => a.id === 'fresh').pct, 0);
  assert.equal(r.find((a) => a.id === 'unknown').pct, null);
});

test('rankAccounts puts an account that is running out itself last', () => {
  const m = new Map(accounts);
  m.set('personal', { ...accounts.get('personal'), forecast: { fiveHourAt: inMin(10) } });
  const r = rankAccounts(m, 'work');
  assert.deepEqual(r.map((a) => a.id), ['fresh', 'side', 'unknown', 'personal']);
  assert.equal(r[3].soon, true);
});

const sess = (handle, account, move, extra = {}) => ({ handle, account, leaving: false, actions: { move, focus: true, stop: true, why: '' }, ...extra });

test('planMoveAll moves every movable session to the account with most room', () => {
  const sessions = [sess('a1', 'work', true), sess('a2', 'work', true), sess('a3', 'work', false), sess('a4', 'work', true, { leaving: true }), sess('b1', 'side', true)];
  const p = planMoveAll(sessions, 'work', accounts);
  assert.equal(p.target.id, 'fresh');
  assert.deepEqual(p.handles, ['a1', 'a2']);
  assert.equal(p.skipped, 1);
});

test('planMoveAll has no target when nothing is movable or nowhere to go', () => {
  assert.equal(planMoveAll([sess('a1', 'work', false)], 'work', accounts).handles.length, 0);
  const lone = new Map([['work', accounts.get('work')]]);
  assert.equal(planMoveAll([sess('a1', 'work', true)], 'work', lone).target, null);
  const allSoon = new Map([['work', accounts.get('work')], ['p', { label: 'P', limits: { fiveHour: { usedPct: 5 }, readAt: 'x' }, forecast: { fiveHourAt: inMin(5) } }]]);
  assert.equal(planMoveAll([sess('a1', 'work', true)], 'work', allSoon).target, null);
});

test('confirm texts name the project, account and target', () => {
  assert.equal(confirmStop('clalt', 'WORK'), 'Stop clalt on WORK? It ends after the current step.');
  assert.equal(confirmMoveAll(2, 'WORK', { label: 'PERSONAL', pct: 12 }), 'Move 2 sessions from WORK to PERSONAL (12%)? Each moves at its next pause.');
  assert.equal(confirmMoveAll(1, 'WORK', { label: 'FRESH', pct: 0 }), 'Move 1 session from WORK to FRESH (0%)? It moves at its next pause.');
});

test('moveRequest is a same origin json post with the edit header', () => {
  const r = moveRequest('move', { handle: 'h', account: 'p' });
  assert.equal(r.url, '/api/session/move');
  assert.equal(r.init.method, 'POST');
  assert.equal(r.init.credentials, 'same-origin');
  assert.equal(r.init.headers['X-Clalt-Edit'], '1');
  assert.equal(r.init.headers['Content-Type'], 'application/json');
  assert.deepEqual(JSON.parse(r.init.body), { handle: 'h', account: 'p' });
  assert.deepEqual(JSON.parse(moveRequest('stop', { handle: 'h' }).init.body), { handle: 'h' });
});

test('sendAction returns ok or the reason of a refusal', async () => {
  const reply = (status, body) => async () => ({ ok: status < 300, status, json: async () => body });
  assert.deepEqual(await sendAction('stop', { handle: 'h' }, reply(202, { stopping: true })), { ok: true });
  assert.deepEqual(await sendAction('move', { handle: 'h', account: 'p' }, reply(409, { reason: 'the session has ended' })), { ok: false, reason: 'the session has ended' });
  assert.match((await sendAction('stop', { handle: 'h' }, reply(401, {}))).reason, /lost its access/);
  assert.match((await sendAction('stop', { handle: 'h' }, reply(500, { reason: 'that did not work' }))).reason, /did not work/);
  assert.match((await sendAction('stop', { handle: 'h' }, async () => { throw new TypeError('x'); })).reason, /not reachable/);
  assert.match((await sendAction('stop', { handle: 'h' }, async () => ({ ok: false, status: 409, json: async () => { throw new Error('bad'); } }))).reason, /http 409/);
});
