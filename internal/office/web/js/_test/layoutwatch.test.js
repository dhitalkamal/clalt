// layout loading and the rev watcher, with a fake fetch and a fake clock.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createLayoutWatcher } from '../layoutwatch.js';
import { loadLayout, loadFirst } from '../layoutload.js';

const V2 = (name) => ({ v: 2, name, site: {}, pavilions: [], overflow: [] });
const reply = (rev, name = 'river') => ({ edit: false, layout: V2(name), templates: [{ id: name, title: name }], rev });

function rig(replies, every = 3000) {
  let clock = 0;
  const calls = [];
  const applied = [];
  const load = async (template) => { calls.push(template); const r = replies.shift(); if (r instanceof Error) throw r; return r; };
  const w = createLayoutWatcher({ load, apply: async (body) => { applied.push(body.rev); }, every, now: () => clock });
  return { w, calls, applied, tick: (ms) => { clock += ms; } };
}

test('select loads and applies the template at once', async () => {
  const r = rig([reply('r1')]);
  await r.w.select('river');
  assert.deepEqual(r.calls, ['river']);
  assert.deepEqual(r.applied, ['r1']);
});

test('check re-applies only when the rev changes', async () => {
  const r = rig([reply('r1'), reply('r1'), reply('r2')]);
  await r.w.select('river');
  r.tick(3000); await r.w.check();
  assert.deepEqual(r.applied, ['r1']);
  r.tick(3000); await r.w.check();
  assert.deepEqual(r.applied, ['r1', 'r2']);
});

test('check waits for the cadence and never overlaps itself', async () => {
  const r = rig([reply('r1'), reply('r2')]);
  await r.w.select('river');
  r.tick(1000); await r.w.check();
  assert.equal(r.calls.length, 1);
  r.tick(3000);
  await Promise.all([r.w.check(), r.w.check()]);
  assert.equal(r.calls.length, 2);
});

test('select applies even when the rev is the same as the old template rev', async () => {
  const r = rig([reply('r1', 'river'), reply('r1', 'rows')]);
  await r.w.select('river');
  await r.w.select('rows');
  assert.deepEqual(r.applied, ['r1', 'r1']);
});

test('a failed check is kept quiet and tried again later', async () => {
  const r = rig([reply('r1'), new Error('http 500'), reply('r2')]);
  await r.w.select('river');
  r.tick(3000); await r.w.check();
  r.tick(3000); await r.w.check();
  assert.deepEqual(r.applied, ['r1', 'r2']);
});

test('an answer for a template that is no longer selected is dropped', async () => {
  let release;
  const slow = new Promise((res) => { release = res; });
  const applied = [];
  const load = (t) => (t === 'river' ? slow : Promise.resolve(reply('r9', t)));
  const w = createLayoutWatcher({ load, apply: async (b) => { applied.push(b.layout.name); }, every: 0, now: () => 0 });
  const first = w.select('river');
  await new Promise((r) => setImmediate(r));
  const second = w.select('rows');
  release(reply('r1', 'river'));
  await Promise.all([first, second]);
  assert.deepEqual(applied, ['rows']);
});

function stubFetch(map) {
  const seen = [];
  globalThis.fetch = async (url) => {
    seen.push(String(url));
    const v = map[String(url)];
    if (v === undefined) return { ok: false, status: 404, json: async () => ({}) };
    return { ok: true, status: 200, json: async () => v };
  };
  return seen;
}

test('loadLayout asks for the template and returns edit, layout, templates, rev', async () => {
  const seen = stubFetch({ '/api/layout?template=river': { edit: true, layout: V2('river'), templates: [{ id: 'river', title: 'River' }], rev: 'abc' } });
  const got = await loadLayout('river');
  assert.deepEqual(seen, ['/api/layout?template=river']);
  assert.equal(got.edit, true);
  assert.equal(got.rev, 'abc');
  assert.equal(got.layout.v, 2);
  assert.equal(got.templates[0].id, 'river');
});

test('loadLayout without a template asks for the server default', async () => {
  const seen = stubFetch({ '/api/layout': { edit: false, layout: V2('rows'), templates: [], rev: '1' } });
  await loadLayout();
  assert.deepEqual(seen, ['/api/layout']);
});

test('loadLayout still takes a v1 hall and a bare layout', async () => {
  const hall = { v: 1, size: { w: 1, d: 1 }, rooms: [], furniture: [] };
  stubFetch({ '/api/layout': { edit: false, layout: hall } });
  assert.equal((await loadLayout()).layout.v, 1);
  stubFetch({ '/api/layout': hall });
  assert.equal((await loadLayout()).edit, false);
});

test('loadLayout throws on a failed fetch and carries the status', async () => {
  stubFetch({});
  await assert.rejects(loadLayout('river'), (e) => e.status === 404);
});

test('loadLayout throws on a 400 for an unknown template', async () => {
  globalThis.fetch = async () => ({ ok: false, status: 400, json: async () => ({}) });
  await assert.rejects(loadLayout('nope'), (e) => e.status === 400);
});

test('loadLayout throws on an answer that is not a layout', async () => {
  stubFetch({ '/api/layout': { edit: true, layout: { v: 9 } } });
  await assert.rejects(loadLayout());
});

const HALL = { v: 1, size: { w: 1, d: 1 }, rooms: [], furniture: [] };

test('loadFirst falls back to the embedded hall when the server fails', async () => {
  stubFetch({ './layout/default.json': HALL });
  const got = await loadFirst();
  assert.equal(got.layout.v, 1);
  assert.equal(got.edit, false);
  assert.deepEqual(got.templates, []);
  assert.equal(got.template, null);
});

test('loadFirst without a template falls back to the hall on failure', async () => {
  stubFetch({ './layout/default.json': HALL });
  assert.equal((await loadFirst()).layout.v, 1);
});

test('select rejects when the load fails and the template stays', async () => {
  const r = rig([reply('r1'), new Error('http 400')]);
  await r.w.select('river');
  await assert.rejects(r.w.select('foo'));
  assert.equal(r.w.template, 'river');
  assert.deepEqual(r.applied, ['r1']);
});

test('a failed check keeps the scene and the rev', async () => {
  const r = rig([reply('r1'), new Error('http 401'), reply('r1')]);
  await r.w.select('river');
  r.tick(3000); await r.w.check();
  r.tick(3000); await r.w.check();
  assert.deepEqual(r.applied, ['r1']);
});

test('applies never overlap and only the latest request lands', async () => {
  let active = 0;
  let peak = 0;
  const applied = [];
  const gates = [];
  const apply = async (b) => {
    active++; peak = Math.max(peak, active);
    await new Promise((res) => gates.push(res));
    applied.push(b.layout.name);
    active--;
  };
  const load = async (t) => reply('r', t);
  const w = createLayoutWatcher({ load, apply, every: 0, now: () => 0 });
  const p1 = w.select('river');
  await new Promise((r) => setImmediate(r));
  const p2 = w.select('rows');
  const p3 = w.select('street');
  await new Promise((r) => setImmediate(r));
  gates.shift()();
  await new Promise((r) => setImmediate(r));
  while (gates.length) { gates.shift()(); await new Promise((r) => setImmediate(r)); }
  await Promise.all([p1, p2, p3]);
  assert.equal(peak, 1);
  assert.deepEqual(applied, ['river', 'street']);
  assert.equal(w.template, 'street');
});

test('loadLayout url-encodes the template id', async () => {
  const seen = stubFetch({});
  await loadLayout('a b').catch(() => {});
  assert.equal(seen[0], '/api/layout?template=a%20b');
});

test('adopt sets the baseline so an unchanged rev is not applied again', async () => {
  const r = rig([reply('r1'), reply('r2')]);
  r.w.adopt('river', 'r1');
  r.tick(3000); await r.w.check();
  assert.deepEqual(r.applied, []);
  r.tick(3000); await r.w.check();
  assert.deepEqual(r.applied, ['r2']);
  assert.equal(r.w.template, 'river');
});
