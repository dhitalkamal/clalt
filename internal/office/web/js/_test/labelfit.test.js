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
  const x = it.x + (r.dx || 0);
  return { x0: x - w / 2, x1: x + w / 2, y0: y - h, y1: y };
}
// the shape callers care about for a label that stayed put
const still = (r) => ({ dy: r.dy, compact: r.compact });
const hit = (a, b) => a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1;

function overlaps(items, out) {
  const bad = [];
  items = items.filter((it) => !out.get(it.id).hidden);
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
  for (const it of items) assert.deepEqual(still(out.get(it.id)), { dy: 0, compact: false });
});

test('two cards on top of each other: the one higher on screen moves up, no overlap', () => {
  const items = [card('a', 100, 200), card('b', 110, 180)];
  const out = fitLabels(items);
  assert.deepEqual(still(out.get('a')), { dy: 0, compact: false });
  assert.ok(out.get('b').dy < 0 || out.get('b').dx !== 0);
  assert.deepEqual(overlaps(items, out), []);
});

test('a crowd that cannot be nudged apart turns into compact cards', () => {
  const items = [];
  for (let i = 0; i < 8; i++) items.push(card('c' + i, 300 + i * 10, 400 + i * 4));
  const out = fitLabels(items);
  assert.ok([...out.values()].some((r) => r.compact));
  for (const r of out.values()) assert.ok(Math.hypot(r.dx, r.dy) <= 240);
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
  assert.deepEqual(still(out.get('k')), { dy: 0, compact: false });
  assert.ok(out.get('p').dy !== 0 || out.get('p').dx !== 0 || out.get('p').hidden);
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
  assert.deepEqual(still(fitLabels([card('a', 1, 1)]).get('a')), { dy: 0, compact: false });
});

// eight account cards plus room pills packed into a small area, like a village at iso
function dense(seed = 1, n = 8, pills = 10) {
  let s = seed;
  const rnd = () => ((s = (s * 1664525 + 1013904223) % 4294967296) / 4294967296);
  const items = [];
  for (let i = 0; i < n; i++) items.push(card('c' + i, 500 + rnd() * 260, 330 + rnd() * 120));
  for (let i = 0; i < pills; i++) items.push(pill('p' + i, 480 + rnd() * 300, 300 + rnd() * 170));
  return items;
}

test('dense card and pill sets never overlap, cards are never hidden', () => {
  for (let seed = 1; seed <= 60; seed++) {
    const items = dense(seed);
    const out = fitLabels(items);
    assert.deepEqual(overlaps(items, out), [], 'seed ' + seed);
    for (const it of items) if (it.id[0] === 'c') assert.equal(out.get(it.id).hidden, false);
  }
});

test('cards stacked on one anchor still end clear', () => {
  const items = [];
  for (let i = 0; i < 8; i++) items.push(card('c' + i, 600, 400));
  const out = fitLabels(items);
  assert.deepEqual(overlaps(items, out), []);
});

test('a card moved more than 24px carries a leader, one that stayed does not', () => {
  const items = dense(3);
  const out = fitLabels(items);
  for (const it of items) {
    const r = out.get(it.id);
    assert.equal(!!r.leader, Math.hypot(r.dx, r.dy) > 24 && !r.hidden, it.id);
  }
  assert.ok([...out.values()].some((r) => r.leader));
});

test('tiny anchor moves between frames do not reshuffle the layout', () => {
  for (let seed = 1; seed <= 20; seed++) {
    const a = dense(seed);
    const one = fitLabels(a);
    // the whole scene drifts by under a pixel, as it does while the camera eases
    const b = a.map((it, i) => ({ ...it, x: it.x + 0.4 + (i % 3) * 0.2, y: it.y - 0.3 }));
    const two = fitLabels(b, { prev: one });
    for (const it of a) {
      const p = one.get(it.id);
      const q = two.get(it.id);
      assert.equal(q.compact, p.compact, it.id + ' seed ' + seed);
      assert.equal(!!q.hidden, !!p.hidden, it.id + ' seed ' + seed);
      assert.ok(Math.hypot(q.dx - p.dx, q.dy - p.dy) <= 2, `${it.id} seed ${seed}`);
    }
  }
});

test('with no hint the same input gives the same answer', () => {
  const a = dense(7);
  const one = fitLabels(a);
  const two = fitLabels([...a].reverse());
  for (const it of a) assert.deepEqual(one.get(it.id), two.get(it.id));
});

test('pills that cannot find room nearby are hidden, never cards', () => {
  const items = [];
  for (let i = 0; i < 6; i++) items.push(card('c' + i, 400 + i * 60, 300));
  for (let i = 0; i < 30; i++) items.push(pill('p' + i, 500, 300));
  const out = fitLabels(items);
  assert.deepEqual(overlaps(items, out), []);
  assert.ok([...out.entries()].some(([id, r]) => id[0] === 'p' && r.hidden));
  for (const it of items) if (it.id[0] === 'c') assert.equal(out.get(it.id).hidden, false);
});
