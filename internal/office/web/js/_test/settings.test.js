// settings model: option list from /api/layouts, the save request and the error text. pure, no dom.
import test from 'node:test';
import assert from 'node:assert/strict';
import { layoutOptions, saveRequest, errorText, saveDefault, SAVE_LABEL, confirmText, canSetDefault } from '../settings.js';

const REPLY = {
  active: 'river',
  layouts: [
    { name: 'default', title: 'Default', builtin: true },
    { name: 'river', title: 'River', builtin: true },
    { name: 'mine', title: 'My floor', builtin: false },
    { name: 'plain', title: '', builtin: false },
  ],
  templates: [{ id: 'rows', title: 'Rows' }, { id: 'river', title: 'River' }, { id: 'hall', title: 'Hall' }],
};

test('layoutOptions lists templates then saved layouts, once each, current one checked', () => {
  const { options, active } = layoutOptions(REPLY);
  assert.equal(active, 'river');
  assert.deepEqual(options.map((o) => o.id), ['rows', 'river', 'hall', 'mine', 'plain']);
  assert.deepEqual(options.filter((o) => o.checked).map((o) => o.id), ['river']);
  assert.equal(options.find((o) => o.id === 'mine').title, 'My floor');
  assert.equal(options.find((o) => o.id === 'plain').title, 'plain');
  assert.equal(options.find((o) => o.id === 'rows').group, 'template');
  assert.equal(options.find((o) => o.id === 'mine').group, 'saved');
});

test('layoutOptions checks a saved layout and survives odd replies', () => {
  assert.equal(layoutOptions({ ...REPLY, active: 'mine' }).options.find((o) => o.checked).id, 'mine');
  assert.equal(layoutOptions({ ...REPLY, active: 'gone' }).options.some((o) => o.checked), false);
  assert.deepEqual(layoutOptions(null).options, []);
  assert.deepEqual(layoutOptions({ active: 'x' }).options, []);
  assert.deepEqual(layoutOptions({ layouts: [null, { title: 'no name' }, { name: 'ok' }] }).options.map((o) => o.id), ['ok']);
});

test('saveRequest is a same origin json post with the edit header', () => {
  const r = saveRequest('river');
  assert.equal(r.url, '/api/layout/use');
  assert.equal(r.init.method, 'POST');
  assert.equal(r.init.credentials, 'same-origin');
  assert.equal(r.init.headers['Content-Type'], 'application/json');
  assert.equal(r.init.headers['X-Clalt-Edit'], '1');
  assert.deepEqual(JSON.parse(r.init.body), { name: 'river' });
});

test('errorText says what failed in one line', () => {
  assert.equal(errorText({ status: 404, text: 'no such layout\n' }), 'could not save the default office: no such layout');
  assert.equal(errorText({ status: 403, text: '' }), 'could not save the default office: http 403');
  assert.equal(errorText({ network: true }), 'could not save the default office: clalt office is not reachable');
});

test('saveDefault posts and returns the body, or throws the error text', async () => {
  let seen;
  const ok = async (url, init) => { seen = { url, init }; return { ok: true, status: 200, json: async () => ({ layout: {} }) }; };
  assert.deepEqual(await saveDefault('rows', ok), { layout: {} });
  assert.equal(seen.url, '/api/layout/use');
  const bad = async () => ({ ok: false, status: 404, text: async () => 'no such layout\n' });
  await assert.rejects(saveDefault('x', bad), { message: 'could not save the default office: no such layout' });
  const down = async () => { throw new TypeError('failed to fetch'); };
  await assert.rejects(saveDefault('x', down), { message: 'could not save the default office: clalt office is not reachable' });
});

test('builtin files are left out and an active default shows as hall', () => {
  const { options } = layoutOptions({ ...REPLY, active: 'default' });
  assert.equal(options.some((o) => o.id === 'default'), false);
  assert.deepEqual(options.filter((o) => o.checked).map((o) => o.id), ['hall']);
});

test('the button says what it does', () => {
  assert.equal(SAVE_LABEL, 'Set as default');
});

test('the default option is flagged, only that one', () => {
  const { options } = layoutOptions(REPLY);
  assert.deepEqual(options.filter((o) => o.isDefault).map((o) => o.id), ['river']);
});

test('confirmText names the new default and says when it opens', () => {
  assert.equal(confirmText('River'), 'River is now your default. It opens every time you run clalt office.');
});

test('canSetDefault is false when nothing is picked or the pick is the default', () => {
  assert.equal(canSetDefault('river', 'river'), false);
  assert.equal(canSetDefault('rows', 'river'), true);
  assert.equal(canSetDefault(null, 'river'), false);
});

// start at login
import { autostartModel, autostartRequest, autostartErrorText, loadAutostart, saveAutostart, AUTOSTART_LABEL, AUTOSTART_UNSUPPORTED } from '../settings.js';

test('autostartModel: supported shows the state, unsupported is disabled with the reason', () => {
  assert.deepEqual(autostartModel({ supported: true, enabled: true }), { supported: true, enabled: true, disabled: false, note: '' });
  assert.deepEqual(autostartModel({ supported: true, enabled: false }), { supported: true, enabled: false, disabled: false, note: '' });
  const off = autostartModel({ supported: false, enabled: true });
  assert.equal(off.disabled, true);
  assert.equal(off.enabled, false);
  assert.equal(off.note, 'start at login is macOS only for now');
  assert.equal(AUTOSTART_UNSUPPORTED, off.note);
  assert.equal(autostartModel(null).disabled, true);
  assert.equal(AUTOSTART_LABEL, 'Start office when I log in');
});

test('autostartRequest posts json with the edit header and same-origin credentials', () => {
  const { url, init } = autostartRequest(true);
  assert.equal(url, '/api/office/autostart');
  assert.equal(init.method, 'POST');
  assert.equal(init.headers['Content-Type'], 'application/json');
  assert.equal(init.headers['X-Clalt-Edit'], '1');
  assert.equal(init.credentials, 'same-origin');
  assert.deepEqual(JSON.parse(init.body), { enabled: true });
  assert.deepEqual(JSON.parse(autostartRequest(false).init.body), { enabled: false });
});

test('autostartErrorText says what failed in one line', () => {
  assert.equal(autostartErrorText({ network: true }), 'could not change start at login: clalt office is not reachable');
  assert.equal(autostartErrorText({ status: 500, text: ' boom \n' }), 'could not change start at login: boom');
  assert.equal(autostartErrorText({ status: 403, text: '' }), 'could not change start at login: http 403');
});

test('loadAutostart and saveAutostart return the model, failures carry a ready message', async () => {
  const ok = (body) => async () => ({ ok: true, json: async () => body });
  assert.equal((await loadAutostart(ok({ supported: true, enabled: true }))).enabled, true);
  assert.equal((await saveAutostart(false, ok({ supported: true, enabled: false }))).enabled, false);
  await assert.rejects(saveAutostart(true, async () => ({ ok: false, status: 400, text: async () => 'macOS only' })), /could not change start at login: macOS only/);
  await assert.rejects(saveAutostart(true, async () => { throw new Error('x'); }), /not reachable/);
  await assert.rejects(loadAutostart(async () => ({ ok: false, status: 404, text: async () => '' })), /404/);
});
