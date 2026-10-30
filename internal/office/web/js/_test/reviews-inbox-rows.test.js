// the pr reviews panel row: retry state shown on the row, retry text styled as muted secondary text.
// node has no dom, so a tiny fake document is installed before inbox.js is imported.
process.env.TZ = 'UTC';
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { tabRows } from '../reviews/model.js';

// minimal fake element: enough for el() in dom.js and the row builder
class Text {
  constructor(s) { this.data = String(s); }
  get textContent() { return this.data; }
}
class Fake {
  constructor(tag) {
    this.tagName = tag;
    this.className = '';
    this.children = [];
    this.attrs = {};
    this.handlers = {};
    this.hidden = false;
    this.type = '';
    this.title = '';
  }
  append(...kids) { for (const k of kids) this.children.push(typeof k === 'string' ? new Text(k) : k); }
  replaceChildren() { this.children = []; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  addEventListener(ev, fn) { (this.handlers[ev] ||= []).push(fn); }
  click() { for (const fn of this.handlers.click || []) fn({ type: 'click' }); }
  get textContent() { return this.children.map((c) => c.textContent).join(''); }
  set textContent(s) { this.children = [new Text(s)]; }
}
globalThis.document = { createElement: (tag) => new Fake(tag), body: new Fake('body') };

// namespace import so a missing export fails as an assertion, not a link error
const inbox = await import('../reviews/inbox.js');
const rowNode = (...a) => {
  assert.equal(typeof inbox.rowNode, 'function', 'rowNode is exported from inbox.js');
  return inbox.rowNode(...a);
};

const NOW = Date.parse('2026-10-27T12:00:00Z');
const row = (extra) => ({ id: 'o-r-4-ddddddd', repo: 'o/r', number: 4, title: 'Bad', author: 'di', endedAt: '2026-10-27T10:00:00Z', counts: { high: 0, medium: 0, low: 0 }, handle: 'ext-s4', ...extra });
const nodeOf = (extra, onOpen = () => {}) => {
  const reply = { reviews: [{ state: 'open', ...row(extra) }], maxAttempts: 5 };
  return rowNode(tabRows(reply, NOW).open[0], onOpen);
};
// all descendants with a given class token
const withClass = (node, cls) => {
  const out = [];
  for (const c of node.children) {
    if (!c.children) continue;
    if (c.className.split(' ').includes(cls)) out.push(c);
    out.push(...withClass(c, cls));
  }
  return out;
};

test('failed row with a pending retry renders one retry span next to the error', () => {
  const node = nodeOf({ error: 'claude failed', attempts: 2, nextRetryAt: '2026-10-27T13:05:00Z' });
  const retry = withClass(node, 'retry');
  assert.equal(retry.length, 1);
  assert.equal(retry[0].textContent, 'retrying at 13:05 (attempt 3 of 5)');
  const sum = withClass(node, 'sum');
  assert.equal(sum.length, 1);
  assert.equal(sum[0].textContent, 'claude failed');
});

test('row is a button with the item and tone classes and itl, meta, tail children', () => {
  const node = nodeOf({ error: 'claude failed', attempts: 2, nextRetryAt: '2026-10-27T13:05:00Z' });
  assert.equal(node.tagName, 'button');
  assert.equal(node.className, 'item error');
  assert.deepEqual(node.children.map((c) => c.className), ['itl', 'meta', 'tail']);
});

test('successful row has no retry span, even with stray attempts and nextRetryAt', () => {
  const node = nodeOf({ attempts: 2, nextRetryAt: '2026-10-27T13:05:00Z' });
  assert.equal(withClass(node, 'retry').length, 0);
});

test('given-up row has no retry span and its sum is the error', () => {
  const err = 'claude exited 1: limit reached; gave up after 5 attempts';
  const node = nodeOf({ error: err, attempts: 5 });
  assert.equal(withClass(node, 'retry').length, 0);
  assert.equal(withClass(node, 'sum')[0].textContent, err);
});

test('clicking the row calls onOpen with the row id', () => {
  const seen = [];
  const node = nodeOf({ error: 'x', attempts: 1 }, (id) => seen.push(id));
  node.click();
  assert.deepEqual(seen, ['o-r-4-ddddddd']);
});

test('css: .reviews .retry is muted with the same color as meta and sum', () => {
  const css = fs.readFileSync(new URL('../../reviews.css', import.meta.url), 'utf8');
  const rule = css.match(/(^|\n)\s*\.reviews \.retry\s*\{([^}]*)\}/);
  assert.ok(rule, 'a .reviews .retry rule exists');
  assert.match(rule[2], /(^|[\s;])color:\s*var\(--nb-pencil\)/);
});
