// the page shows the server's active layout: no template in the hash, storage or the request
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadFirst } from '../layoutload.js';
import * as view from '../viewstate.js';

const js = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const src = (f) => readFileSync(resolve(js, f), 'utf8');

test('loadFirst asks for the active layout, never a template', async () => {
  const urls = [];
  globalThis.fetch = async (url) => {
    urls.push(String(url));
    return { ok: true, status: 200, json: async () => ({ edit: true, layout: { v: 2, site: {}, pavilions: [] }, templates: [], rev: 'a' }) };
  };
  const first = await loadFirst();
  assert.deepEqual(urls, ['/api/layout']);
  assert.equal(first.template, null);
  assert.equal(first.edit, true);
});

test('viewstate keeps no template pick', () => {
  for (const name of ['pickTemplate', 'remember', 'recall', 'orderTemplates']) assert.equal(name in view, false, name);
  assert.deepEqual(view.parseHash('#template=river&cam=hero'), { cam: 'sky' });
  assert.equal(view.buildHash({ cam: 'ground' }), '#cam=ground');
  assert.equal(view.buildHash({ cam: null }), '');
});

test('boot and view wiring do not read the hash template or localStorage', () => {
  for (const f of ['main.js', 'viewctl.js', 'chrome.js']) {
    const s = src(f);
    assert.equal(/localStorage|pickTemplate|recall\(|template=/.test(s), false, f);
  }
});
