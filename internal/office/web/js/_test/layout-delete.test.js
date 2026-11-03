// delete a saved layout: the request, the error text and the press twice state. pure, no dom.
import test from 'node:test';
import assert from 'node:assert/strict';
import { deleteRequest, deleteErrorText, deleteLayout, deletedNote, canDelete, DELETE_LABEL, CONFIRM_LABEL } from '../settings.js';
import { createConfirmPress } from '../confirmpress.js';

test('deleteRequest is a same origin json post with the edit header', () => {
  const r = deleteRequest('hall-custom');
  assert.equal(r.url, '/api/layout/delete');
  assert.equal(r.init.method, 'POST');
  assert.equal(r.init.credentials, 'same-origin');
  assert.equal(r.init.headers['Content-Type'], 'application/json');
  assert.equal(r.init.headers['X-Clalt-Edit'], '1');
  assert.deepEqual(JSON.parse(r.init.body), { name: 'hall-custom' });
});

test('deleteErrorText says what failed in one line', () => {
  assert.equal(deleteErrorText({ status: 403, text: 'built-in layouts cannot be deleted\n' }), 'could not delete the layout: built-in layouts cannot be deleted');
  assert.equal(deleteErrorText({ status: 500, text: '' }), 'could not delete the layout: http 500');
  assert.equal(deleteErrorText({ network: true }), 'could not delete the layout: clalt office is not reachable');
});

test('labels and the note', () => {
  assert.equal(DELETE_LABEL, 'Delete');
  assert.equal(CONFIRM_LABEL, 'Delete layout?');
  assert.equal(deletedNote('My floor'), 'Deleted My floor.');
});

test('only saved layouts can be deleted', () => {
  assert.equal(canDelete({ group: 'saved' }), true);
  assert.equal(canDelete({ group: 'template' }), false);
  assert.equal(canDelete(null), false);
});

test('deleteLayout resolves to the new list and throws a ready message', async () => {
  const ok = async (url, init) => ({ ok: true, json: async () => ({ active: 'rows', url, body: init.body }) });
  const body = await deleteLayout('mine', ok);
  assert.equal(body.active, 'rows');
  assert.equal(body.url, '/api/layout/delete');
  const no = async () => ({ ok: false, status: 403, text: async () => 'built-in layouts cannot be deleted\n' });
  await assert.rejects(deleteLayout('rows', no), { message: 'could not delete the layout: built-in layouts cannot be deleted' });
  const down = async () => { throw new TypeError('failed'); };
  await assert.rejects(deleteLayout('x', down), { message: 'could not delete the layout: clalt office is not reachable' });
});

// a fake timer: fire() runs the pending callback
function fake() {
  let cb = null;
  return {
    set: (f) => { cb = f; return 1; },
    clear: () => { cb = null; },
    fire: () => { const f = cb; cb = null; if (f) f(); },
    pending: () => cb !== null,
  };
}

test('the first press arms, the second confirms', () => {
  const t = fake();
  const seen = [];
  const p = createConfirmPress({ setTimer: t.set, clearTimer: t.clear, onChange: (a) => seen.push(a) });
  assert.equal(p.armed, false);
  assert.equal(p.press(), 'arm');
  assert.equal(p.armed, true);
  assert.equal(t.pending(), true);
  assert.equal(p.press(), 'confirm');
  assert.equal(p.armed, false);
  assert.equal(t.pending(), false);
  assert.deepEqual(seen, [true, false]);
});

test('the timeout, reset and a new arm all go back to idle', () => {
  const t = fake();
  const p = createConfirmPress({ setTimer: t.set, clearTimer: t.clear });
  p.press();
  t.fire();
  assert.equal(p.armed, false);
  p.press();
  p.reset();
  assert.equal(p.armed, false);
  assert.equal(t.pending(), false);
  assert.equal(p.press(), 'arm');
  p.reset();
  p.reset();
  assert.equal(p.armed, false);
});
