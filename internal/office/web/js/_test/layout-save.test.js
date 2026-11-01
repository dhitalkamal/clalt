// the editor's saver: keeps the layout version, sends If-Match, turns a 409 into a conflict error
import test from 'node:test';
import assert from 'node:assert/strict';
import { createLayoutSaver } from '../layoutsave.js';
import { LOST_ACCESS } from '../lostaccess.js';

const reply = (status, body, tag) => ({
  ok: status >= 200 && status < 300,
  status,
  headers: { get: (k) => (k.toLowerCase() === 'etag' ? tag || null : null) },
  json: async () => (typeof body === 'string' ? JSON.parse(body) : body),
  text: async () => (typeof body === 'string' ? body : JSON.stringify(body)),
});
const fake = (handler) => {
  const calls = [];
  const f = async (url, init = {}) => { calls.push({ url, init }); return handler(url, init); };
  f.calls = calls;
  return f;
};

test('refresh reads the version of the layout from the ETag', async () => {
  const f = fake(() => reply(200, { layout: { title: 'x' } }, '"v1"'));
  const saver = createLayoutSaver(f);
  const body = await saver.refresh();
  assert.equal(saver.version(), '"v1"');
  assert.equal(body.layout.title, 'x');
});

test('save sends If-Match and keeps the version of the reply', async () => {
  const f = fake((url, init) => (init.method === 'PUT' ? reply(200, {}, '"v2"') : reply(200, {}, '"v1"')));
  const saver = createLayoutSaver(f);
  await saver.refresh();
  await saver.save({ title: 'a' });
  const put = f.calls[1];
  assert.equal(put.url, '/api/layout');
  assert.equal(put.init.headers['If-Match'], '"v1"');
  assert.equal(put.init.headers['X-Clalt-Edit'], '1');
  assert.equal(saver.version(), '"v2"');
  await saver.save({ title: 'b' });
  assert.equal(f.calls[2].init.headers['If-Match'], '"v2"');
});

test('save without a known version sends no If-Match', async () => {
  const f = fake(() => reply(200, {}));
  const saver = createLayoutSaver(f);
  await saver.save({});
  assert.equal('If-Match' in f.calls[0].init.headers, false);
});

test('a 409 with a layout is a conflict that carries theirs and adopts their version', async () => {
  const f = fake((url, init) => (init.method === 'PUT'
    ? reply(409, { error: 'changed', layout: { title: 'theirs' }, version: 'v9' }, '"v9"')
    : reply(200, {}, '"v1"')));
  const saver = createLayoutSaver(f);
  await saver.refresh();
  await assert.rejects(saver.save({ title: 'mine' }), (e) => {
    assert.equal(e.conflict, true);
    assert.equal(e.theirs.title, 'theirs');
    assert.match(e.message, /another tab/);
    return true;
  });
  assert.equal(saver.version(), '"v9"');
});

test('keep mine is a plain save on the adopted version', async () => {
  let puts = 0;
  const f = fake((url, init) => {
    if (init.method !== 'PUT') return reply(200, {}, '"v1"');
    puts++;
    return puts === 1 ? reply(409, { layout: {}, version: 'v9' }, '"v9"') : reply(200, {}, '"v10"');
  });
  const saver = createLayoutSaver(f);
  await saver.refresh();
  await assert.rejects(saver.save({}));
  await saver.save({});
  assert.equal(f.calls[2].init.headers['If-Match'], '"v9"');
  assert.equal(saver.version(), '"v10"');
});

test('a 409 with a plain text body is an ordinary error, no conflict', async () => {
  const f = fake(() => reply(409, 'layout foo is format v2, this edit is v1; it was not overwritten'));
  const saver = createLayoutSaver(f);
  await assert.rejects(saver.save({}), (e) => !e.conflict && /format v2/.test(e.message));
});

test('another failure throws its one-line text and keeps the version', async () => {
  const f = fake((url, init) => (init.method === 'PUT' ? reply(400, 'bad layout\n') : reply(200, {}, '"v1"')));
  const saver = createLayoutSaver(f);
  await saver.refresh();
  await assert.rejects(saver.save({}), /bad layout/);
  assert.equal(saver.version(), '"v1"');
});

test('adopt takes the version of any reply that has one', async () => {
  const saver = createLayoutSaver(fake(() => reply(200, {})));
  saver.adopt(reply(200, {}, '"v7"'));
  assert.equal(saver.version(), '"v7"');
  saver.adopt(reply(200, {}));
  assert.equal(saver.version(), '"v7"');
});

test('a 401 on save or refresh says the page lost its access, not unauthorized', async () => {
  const saver = createLayoutSaver(fake(() => reply(401, 'unauthorized\n')));
  await assert.rejects(saver.refresh(), (e) => e.message === LOST_ACCESS);
  await assert.rejects(saver.save({}), (e) => e.message === LOST_ACCESS);
});
