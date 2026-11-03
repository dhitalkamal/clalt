// the shared rules store: loads defaults on a 404, saves the whole object, reverts on an error
import test from 'node:test';
import assert from 'node:assert/strict';
import { createIdleStore } from '../idle/store.js';
import { DEFAULT_RULES } from '../idle/resolve.js';

const reply = (status, body) => ({ ok: status >= 200 && status < 300, status, json: async () => body, text: async () => (typeof body === 'string' ? body : JSON.stringify(body)) });
const fake = (handler) => {
  const calls = [];
  const f = async (url, init = {}) => { calls.push({ url, init }); return handler(url, init, calls.length); };
  f.calls = calls;
  return f;
};

test('a 404 means an older server: defaults, and the feature is not available', async () => {
  const store = createIdleStore(fake(() => reply(404, 'not found')));
  await store.load();
  assert.deepEqual(store.rules(), DEFAULT_RULES);
  assert.equal(store.available(), false);
});

test('a load shows the saved rules and tells subscribers', async () => {
  const saved = { default: 'hide', teamDefault: 'desk', accounts: { a: 'desk' }, teams: {}, subtypes: {} };
  const store = createIdleStore(fake(() => reply(200, saved)));
  const seen = [];
  store.subscribe((r) => seen.push(r));
  await store.load();
  assert.equal(store.available(), true);
  assert.equal(store.rules().default, 'hide');
  assert.equal(seen.length, 1);
});

test('a failed load keeps the defaults and stays available for a retry', async () => {
  const store = createIdleStore(fake(() => { throw new Error('offline'); }));
  await store.load();
  assert.deepEqual(store.rules(), DEFAULT_RULES);
  assert.equal(store.available(), true);
});

test('set posts the whole rules object with the edit header', async () => {
  const f = fake((url, init) => (init.method === 'POST' ? reply(200, JSON.parse(init.body)) : reply(200, DEFAULT_RULES)));
  const store = createIdleStore(f);
  await store.load();
  await store.set('accounts', 'a1', 'hide');
  const post = f.calls[1];
  assert.equal(post.url, '/api/idle');
  assert.equal(post.init.method, 'POST');
  assert.equal(post.init.credentials, 'same-origin');
  assert.equal(post.init.headers['Content-Type'], 'application/json');
  assert.equal(post.init.headers['X-Clalt-Edit'], '1');
  assert.deepEqual(JSON.parse(post.init.body), { ...DEFAULT_RULES, accounts: { a1: 'hide' } });
  assert.equal(store.rules().accounts.a1, 'hide');
});

test('an error keeps the old rules, throws the one-line text and tells nobody', async () => {
  const f = fake((url, init) => (init.method === 'POST' ? reply(400, 'unknown account\n') : reply(200, DEFAULT_RULES)));
  const store = createIdleStore(f);
  await store.load();
  let n = 0;
  store.subscribe(() => n++);
  await assert.rejects(store.set('default', null, 'hide'), /unknown account/);
  assert.equal(store.rules().default, 'break');
  assert.equal(n, 0);
});

test('saves run one after the other so none is lost', async () => {
  let release;
  const gate = new Promise((r) => { release = r; });
  const f = fake(async (url, init) => {
    if (init.method !== 'POST') return reply(200, DEFAULT_RULES);
    if (JSON.parse(init.body).accounts.a) await gate;
    return reply(200, JSON.parse(init.body));
  });
  const store = createIdleStore(f);
  await store.load();
  const one = store.set('accounts', 'a', 'hide');
  const two = store.set('teams', 't', 'break');
  release();
  await Promise.all([one, two]);
  assert.deepEqual(store.rules().accounts, { a: 'hide' });
  assert.deepEqual(store.rules().teams, { t: 'break' });
});

test('unsubscribe stops the calls', async () => {
  const store = createIdleStore(fake(() => reply(200, DEFAULT_RULES)));
  let n = 0;
  const off = store.subscribe(() => n++);
  off();
  await store.load();
  assert.equal(n, 0);
});

const withTag = (status, body, tag) => ({ ...reply(status, body), headers: { get: (k) => (k.toLowerCase() === 'etag' ? tag : null) } });

test('a save sends the version of the last load as If-Match and keeps the new one', async () => {
  const f = fake((url, init) => (init.method === 'POST' ? withTag(200, JSON.parse(init.body), '"v2"') : withTag(200, DEFAULT_RULES, '"v1"')));
  const store = createIdleStore(f);
  await store.load();
  await store.set('accounts', 'a', 'hide');
  assert.equal(f.calls[1].init.headers['If-Match'], '"v1"');
  await store.set('accounts', 'b', 'desk');
  assert.equal(f.calls[2].init.headers['If-Match'], '"v2"');
});

test('a save without a known version sends no If-Match', async () => {
  const f = fake((url, init) => reply(200, init.method === 'POST' ? JSON.parse(init.body) : DEFAULT_RULES));
  const store = createIdleStore(f);
  await store.load();
  await store.set('default', null, 'hide');
  assert.equal('If-Match' in f.calls[1].init.headers, false);
});

test('a 409 adopts the server rules and applies only the pending change on top', async () => {
  const theirs = { ...DEFAULT_RULES, accounts: { work: 'desk' } };
  let posts = 0;
  const f = fake((url, init) => {
    if (init.method !== 'POST') return withTag(200, DEFAULT_RULES, '"v1"');
    posts++;
    if (posts === 1) return withTag(409, { rules: theirs, version: 'v2' }, '"v2"');
    return withTag(200, JSON.parse(init.body), '"v3"');
  });
  const store = createIdleStore(f);
  await store.load();
  const seen = [];
  store.subscribe((r) => seen.push(r));
  await store.set('accounts', 'personal', 'hide');
  assert.equal(posts, 2);
  const retry = f.calls[f.calls.length - 1].init;
  assert.equal(retry.headers['If-Match'], '"v2"');
  assert.deepEqual(JSON.parse(retry.body).accounts, { work: 'desk', personal: 'hide' });
  assert.deepEqual(store.rules().accounts, { work: 'desk', personal: 'hide' });
  assert.deepEqual(seen[seen.length - 1].accounts, { work: 'desk', personal: 'hide' });
});

test('repeated conflicts stop after a few tries with a clear error', async () => {
  let n = 0;
  const f = fake((url, init) => {
    if (init.method !== 'POST') return withTag(200, DEFAULT_RULES, '"v1"');
    n++;
    return withTag(409, { rules: DEFAULT_RULES, version: 'v' + (n + 1) }, '"x"');
  });
  const store = createIdleStore(f);
  await store.load();
  await assert.rejects(store.set('default', null, 'hide'), /changed/);
  assert.ok(n >= 2 && n <= 4, 'tries: ' + n);
});

test('a conflict on a queued save does not lose the save behind it', async () => {
  const server = { rules: { ...DEFAULT_RULES, accounts: { work: 'desk' } }, version: 2 };
  const f = fake((url, init) => {
    if (init.method !== 'POST') return withTag(200, DEFAULT_RULES, '"1"');
    if (init.headers['If-Match'] !== '"' + server.version + '"') return withTag(409, { rules: server.rules, version: String(server.version) }, '"' + server.version + '"');
    server.rules = JSON.parse(init.body);
    server.version++;
    return withTag(200, server.rules, '"' + server.version + '"');
  });
  const store = createIdleStore(f);
  await store.load();
  await Promise.all([store.set('accounts', 'a', 'hide'), store.set('teams', 't', 'break')]);
  assert.deepEqual(server.rules.accounts, { work: 'desk', a: 'hide' });
  assert.deepEqual(server.rules.teams, { t: 'break' });
});
