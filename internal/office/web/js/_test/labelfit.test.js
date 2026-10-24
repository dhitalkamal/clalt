// screen-space label de-overlap: pure boxes in, offsets out
import test from 'node:test';
import assert from 'node:assert/strict';
import { fitLabels } from '../labelfit.js';

// a card anchored at (x, y) occupies [x - w/2, y - h, x + w/2, y]
const card = (id, x, y, extra = {}) => ({ id, x, y, w: 132, h: 60, cw: 90, ch: 22, pri: 1, ...extra });
const pill = (id, x, y) => ({ id, x, y, w: 70, h: 20, pri: 0 });

function boxOf(it, r) {
  const w = r.compact ? it.cw : it.w;
  const h = r.compact ? it.ch : it.h;
  const y = it.y + r.dy;
  return { x0: it.x - w / 2, x1: it.x + w / 2, y0: y - h, y1: y };
}
const hit = (a, b) => a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1;

function overlaps(items, out) {
  const bad = [];
  for (let i = 0; i < items.length; i++) {
    for (let j = i + 1; j < items.length; j++) {
      if (hit(boxOf(items[i], out.get(items[i].id)), boxOf(items[j], out.get(items[j].id)))) bad.push([items[i].id, items[j].id]);
    }
  }
  return bad;
}

test('labels that do not touch stay where they are', () => {
  const items = [card('a', 100, 200), card('b', 400, 200), pill('c', 250, 300)];
  const out = fitLabels(items);
  for (const it of items) assert.deepEqual(out.get(it.id), { dy: 0, compact: false });
});

test('two cards on top of each other: the one higher on screen moves up, no overlap', () => {
  const items = [card('a', 100, 200), card('b', 110, 180)];
  const out = fitLabels(items);
  assert.deepEqual(out.get('a'), { dy: 0, compact: false });
  assert.ok(out.get('b').dy < 0);
  assert.deepEqual(overlaps(items, out), []);
});

test('a crowd that cannot be nudged apart turns into compact cards', () => {
  const items = [];
  for (let i = 0; i < 8; i++) items.push(card('c' + i, 300 + i * 10, 400 + i * 4));
  const out = fitLabels(items);
  assert.ok([...out.values()].some((r) => r.compact));
  for (const r of out.values()) assert.ok(Math.abs(r.dy) <= 120);
});

test('pills have no compact form: they only move', () => {
  const items = [pill('a', 100, 100), pill('b', 100, 100), pill('c', 100, 100)];
  const out = fitLabels(items);
  for (const r of out.values()) assert.equal(r.compact, false);
  assert.deepEqual(overlaps(items, out), []);
});

test('cards win over pills for the spot', () => {
  const items = [pill('p', 100, 200), card('k', 100, 200)];
  const out = fitLabels(items);
  assert.deepEqual(out.get('k'), { dy: 0, compact: false });
  assert.ok(out.get('p').dy < 0);
});

test('the result does not depend on the order of the input', () => {
  const items = [card('a', 100, 200), card('b', 130, 190), card('c', 160, 205), pill('p', 120, 215)];
  const one = fitLabels(items);
  const two = fitLabels([...items].reverse());
  for (const it of items) assert.deepEqual(one.get(it.id), two.get(it.id));
});

test('eight village cards in a ring end without overlaps', () => {
  const items = [];
  for (let i = 0; i < 8; i++) {
    const a = (i / 8) * Math.PI * 2;
    items.push(card('v' + i, 700 + Math.cos(a) * 190, 420 + Math.sin(a) * 110));
  }
  const out = fitLabels(items);
  assert.deepEqual(overlaps(items, out), []);
});

test('empty and single inputs work', () => {
  assert.equal(fitLabels([]).size, 0);
  assert.deepEqual(fitLabels([card('a', 1, 1)]).get('a'), { dy: 0, compact: false });
});
