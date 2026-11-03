// after the active layout is deleted an open editor must not keep the deleted layout
import test from 'node:test';
import assert from 'node:assert/strict';
import { reloadEditorAfterDelete } from '../editorsync.js';

const fakeEditor = (open) => {
  const log = [];
  return { log, api: { isOpen: () => open, setLayout: (l) => log.push(['set', l.title]), close: () => log.push(['close']) } };
};

test('an open editor loads the fallback layout', async () => {
  const { api, log } = fakeEditor(true);
  await reloadEditorAfterDelete(Promise.resolve(api), { layout: { v: 1, title: 'Hall' } });
  assert.deepEqual(log, [['set', 'Hall']]);
});

test('a closed editor is reloaded too so a later open starts clean', async () => {
  const { api, log } = fakeEditor(false);
  await reloadEditorAfterDelete(Promise.resolve(api), { layout: { v: 1, title: 'Hall' } });
  assert.deepEqual(log, [['set', 'Hall']]);
});

test('a layout the editor cannot edit closes it', async () => {
  const { api, log } = fakeEditor(true);
  await reloadEditorAfterDelete(Promise.resolve(api), { layout: { v: 2, title: 'Garden' } });
  assert.deepEqual(log, [['close']]);
});

test('no editor or a failed start is fine', async () => {
  await reloadEditorAfterDelete(null, { layout: { v: 1 } });
  await reloadEditorAfterDelete(Promise.reject(new Error('x')), { layout: { v: 1 } });
});
