// settings model: option list from /api/layouts, the save request and the error text. pure, no dom.
import test from 'node:test';
import assert from 'node:assert/strict';
import { layoutOptions, saveRequest, errorText, saveDefault } from '../settings.js';

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
