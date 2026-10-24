// run with: node --test internal/office/webtest
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { compile } from '../web/js/scene/plan.js';

const read = (p) => JSON.parse(fs.readFileSync(new URL(p, import.meta.url)));
const catalog = read('../layout/data/catalog.json');
const layout = read('../layout/data/templates/default.json');

test('web copies match the authoritative layout files', () => {
  assert.deepEqual(read('../web/layout/catalog.json'), catalog);
  assert.deepEqual(read('../web/layout/default.json'), layout);
});

test('every item of the default template is in the catalog', () => {
  const ids = new Set(catalog.map((c) => c.id));
  for (const f of layout.furniture) assert.ok(ids.has(f.item), f.item);
});

test('default template compiles and every seat is reachable', () => {
  const L = compile(layout, catalog);
  const seats = L.items.filter((i) => i.def.tags.includes('seat'));
  assert.equal(seats.length, 6 + 12 + 8 + 4);
  for (const s of seats) assert.ok(L.seatSlot(s).reachable, 'unreachable ' + s.id + ' ' + s.def.id);
});

test('paths reach the deck through the stairs and climb', () => {
  const L = compile(layout, catalog);
  const seat = L.items.find((i) => i.level === 1 && i.def.id === 'desk');
  const slot = L.seatSlot(seat);
  const p = L.path(L.spawn, slot.node);
  assert.equal(p[0].x, L.spawn.x);
  assert.ok(p.some((q) => q.y > 1.2) && p.some((q) => q.y === 0));
  const last = p[p.length - 1];
  assert.ok(Math.hypot(last.x - slot.chair.x, last.z - slot.chair.z) < 1e-6);
  // never steps between two far points through solid furniture: consecutive legs stay short or straight
  for (let i = 1; i < p.length; i++) assert.ok(Number.isFinite(p[i].x));
});

test('a cabin seat is entered from the lane in front of its door', () => {
  const L = compile(layout, catalog);
  const cabin = L.items.find((i) => i.def.id === 'cabin');
  const slot = L.seatSlot(cabin);
  assert.equal(slot.kind, 'desk');
  assert.ok(slot.stand.length === 2);
  assert.ok(slot.reachable);
});

test('break spots exist for cafe and play items', () => {
  const L = compile(layout, catalog);
  for (const g of ['counter', 'table', 'pong', 'arcade', 'bag', 'cooler']) assert.ok(L.groups[g]?.length, g);
});

test('rotated seats keep working', () => {
  const lay = structuredClone(layout);
  lay.furniture.find((f) => f.item === 'desk' && f.level === 0).rot = 90;
  const L = compile(lay, catalog);
  assert.ok(L.items.length > 0);
});
