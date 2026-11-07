// the Copilot team on the page: fixed colour, label text, no usage, left out of move targets and history
import test from 'node:test';
import assert from 'node:assert/strict';
import { COPILOT_ID, COPILOT_TINT, accountLabel, claudeOnly, editorAccounts, isCopilot, toolName } from '../tool.js';
import { wingView } from '../wingview.js';
import { rankAccounts, planMoveAll } from '../actions/plan.js';
import { describeTarget } from '../activity/target.js';
import { regionInfo } from '../regioncard.js';
import { chipClass } from '../notebook.js';
import { summaryModel } from '../summary/model.js';
import { actionView } from '../actions/availability.js';

const PALETTE = [0x4f86c6, 0x58b08a, 0xd98a3d, 0xa67bc9, 0xc9677b, 0x5fb3b3, 0xb5a23c, 0x7a8aa8];
const rgbToLab = (h) => {
  const lin = (c) => { c /= 255; return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4; };
  const r = lin((h >> 16) & 255), g = lin((h >> 8) & 255), b = lin(h & 255);
  const f = (t) => (t > 0.008856 ? Math.cbrt(t) : 7.787 * t + 16 / 116);
  const x = f((0.4124 * r + 0.3576 * g + 0.1805 * b) / 0.95047), y = f(0.2126 * r + 0.7152 * g + 0.0722 * b), z = f((0.0193 * r + 0.1192 * g + 0.9505 * b) / 1.08883);
  return [116 * y - 16, 500 * (x - y), 200 * (y - z)];
};
const dist = (a, b) => Math.hypot(...rgbToLab(a).map((v, i) => v - rgbToLab(b)[i]));

// a fresh copy of model.js, because the tint table is module state
const freshModel = (n) => import('../model.js?copy=' + n);

test('the copilot colour is fixed, outside the palette and clearly apart from every entry', async () => {
  const m = await freshModel('a');
  assert.equal(m.tintFor(COPILOT_ID), COPILOT_TINT);
  assert.ok(!PALETTE.includes(COPILOT_TINT));
  for (const p of PALETTE) assert.ok(dist(COPILOT_TINT, p) >= 25, p.toString(16));
});

test('copilot does not take a palette slot, claude colours never shift', async () => {
  const plain = await freshModel('b');
  const want = ['x', 'y', 'z'].map((id) => plain.tintFor(id));
  const mixed = await freshModel('c');
  const got = [mixed.tintFor('x'), (mixed.tintFor(COPILOT_ID), mixed.tintFor('y')), mixed.tintFor('z')];
  assert.deepEqual(got, want);
  const first = await freshModel('d');
  first.tintFor(COPILOT_ID);
  assert.equal(first.tintFor('x'), want[0]);
});

test('the ring and the agent tint table both know the copilot colour', async () => {
  const m = await freshModel('e');
  assert.equal(m.ringColor({ role: 'main', account: COPILOT_ID }), COPILOT_TINT);
  m.tintFor(COPILOT_ID);
  assert.equal(m.ACCOUNT_TINT[COPILOT_ID], COPILOT_TINT);
});

test('a session record keeps its tool, missing means claude', async () => {
  const { makeSession } = await freshModel('f');
  assert.equal(makeSession({ handle: 'cp-1', tool: 'copilot' }, COPILOT_ID).tool, 'copilot');
  assert.equal(makeSession({ handle: 'h1' }, 'work').tool, 'claude');
});

test('toolName and isCopilot read the tool or the team id', () => {
  assert.equal(toolName({ tool: 'copilot' }), 'copilot');
  assert.equal(toolName({}), 'claude');
  assert.equal(toolName(null), 'claude');
  assert.ok(isCopilot({ id: COPILOT_ID }));
  assert.ok(isCopilot({ account: COPILOT_ID }));
  assert.ok(isCopilot({ tool: 'copilot' }));
  assert.ok(!isCopilot({ id: 'work' }));
});

test('the copilot team reads Copilot, even without an account record', () => {
  const accounts = new Map([[COPILOT_ID, { label: 'Copilot' }], ['work', { label: 'Work' }]]);
  assert.equal(accountLabel(accounts, COPILOT_ID), 'Copilot');
  assert.equal(accountLabel(new Map(), COPILOT_ID), 'Copilot');
  assert.equal(accountLabel(accounts, 'work'), 'Work');
  assert.equal(accountLabel(accounts, 'gone'), 'gone');
  assert.equal(accountLabel(accounts, ''), '-');
});

test('the copilot card has no bars and no percent, a claude card keeps both', () => {
  const cp = wingView('Copilot', { fiveHour: null, sevenDay: null, readAt: null }, COPILOT_ID);
  assert.deepEqual(cp, { label: 'Copilot', bars: false, worst: null, pct: false });
  const cl = wingView('Work', { fiveHour: { usedPct: 40 }, sevenDay: { usedPct: 10 } }, 'work');
  assert.deepEqual(cl, { label: 'Work', bars: true, worst: 40, pct: true });
  assert.equal(wingView('Work', null, 'work').bars, true);
});

test('copilot is never a move target and its sessions are not moved', () => {
  const accounts = new Map([['work', { label: 'Work', limits: null }], [COPILOT_ID, { label: 'Copilot', tool: 'copilot', limits: { fiveHour: null, sevenDay: null, readAt: null } }], ['home', { label: 'Home', limits: null }]]);
  assert.deepEqual(rankAccounts(accounts, 'work').map((a) => a.id), ['home']);
  const sessions = [{ account: COPILOT_ID, handle: 'cp-1', actions: { move: false } }];
  assert.deepEqual(planMoveAll(sessions, COPILOT_ID, accounts).handles, []);
});

test('claudeOnly drops the copilot account by id or tool', () => {
  const list = [{ id: 'work' }, { id: COPILOT_ID }, { id: 'x', tool: 'copilot' }, { id: 'old', tool: 'claude' }];
  assert.deepEqual(claudeOnly(list).map((a) => a.id), ['work', 'old']);
  assert.deepEqual(claudeOnly(null), []);
});

test('the today model leaves the copilot account out', () => {
  const m = summaryModel({ day: 'd', accounts: [{ id: 'work', label: 'Work' }, { id: COPILOT_ID, label: 'Copilot' }] });
  assert.deepEqual(m.accounts.map((a) => a.id), ['work']);
});

test('activity header and region card say Copilot, with the tool named', () => {
  const accounts = new Map([[COPILOT_ID, { label: 'Copilot' }]]);
  const rec = { role: 'main', handle: 'cp-9', tool: 'copilot', account: COPILOT_ID, project: 'p', state: 'busy' };
  const d = describeTarget(rec, accounts, []);
  assert.equal(d.account, 'Copilot');
  assert.equal(d.tool, 'Copilot');
  assert.match(d.title, /^copilot \(main\)/);
  assert.equal(describeTarget({ ...rec, tool: 'claude', account: 'w' }, new Map(), []).tool, '');
  const info = regionInfo({ pavilion: 'c1', x0: 0, x1: 1, z0: 0, z1: 1 }, { pavilions: [{ id: 'c1', account: COPILOT_ID }] }, new Map(), []);
  assert.deepEqual(info.rows[0], ['account', 'Copilot']);
});

test('the copilot chip has its own class, other names still hash', () => {
  assert.equal(chipClass('Copilot'), 'chip a-copilot');
  assert.match(chipClass('Work'), /a-work/);
});

test('copilot actions show disabled with the why text', () => {
  const v = actionView({ move: false, focus: false, stop: false, why: 'a Copilot session: clalt only shows it' });
  for (const k of ['move', 'focus', 'stop']) assert.deepEqual(v[k], { enabled: false, why: 'a Copilot session: clalt only shows it' });
});

test('the layout editor offers the copilot team as a cabin owner, by its label', () => {
  const accounts = new Map([['work', { label: 'Work', tool: 'claude' }], [COPILOT_ID, { label: 'Copilot', tool: 'copilot' }]]);
  assert.deepEqual(editorAccounts(accounts), [{ id: 'work', name: 'Work' }, { id: COPILOT_ID, name: 'Copilot' }]);
  assert.deepEqual(editorAccounts(null), []);
});
