import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { makeGrid, setOpen, findCells, findPath } from '../web/js/scene/nav.js';

function open(n) {
  const g = makeGrid(n, n);
  g.open.fill(1);
  return g;
}

test('a walled-off goal on a big grid returns null fast', () => {
  const g = open(120);
  for (let i = 0; i < 120; i++) { setOpen(g, i, 100, 0); setOpen(g, 100, i, 0); }
  const t = Date.now();
  assert.equal(findCells(g, { x: 2, z: 2 }, { x: 110, z: 110 }), null);
  assert.ok(Date.now() - t < 100, 'took ' + (Date.now() - t) + ' ms');
});

test('a reachable goal still gets a path around a wall', () => {
  const g = open(40);
  for (let z = 0; z < 39; z++) setOpen(g, 20, z, 0);
  const p = findPath(g, { x: 2, z: 2 }, { x: 38, z: 2 });
  assert.ok(p && p.length >= 3);
  assert.ok(p.some((q) => q.z > 38));
});

test('every catalog item has a builder and a solid rule, except the seats the roster layer draws', () => {
  const cat = JSON.parse(fs.readFileSync(new URL('../layout/data/catalog.json', import.meta.url)));
  const src = ['itemsa', 'itemsb'].map((f) => fs.readFileSync(new URL('../web/js/scene/' + f + '.js', import.meta.url), 'utf8')).join('\n');
  const meta = fs.readFileSync(new URL('../web/js/scene/itemmeta.js', import.meta.url), 'utf8');
  for (const c of cat) {
    if (c.tags.includes('seat')) continue;
    assert.ok(new RegExp("(^|\\s)'?" + c.id + "'?[:,]").test(src), 'no builder for ' + c.id);
  }
  // solid rules: items that are not floor/wall tagged either have a META entry or are fully solid by default;
  // the ones with an open part must be listed
  for (const id of ['round-table', 'coffee-bar', 'high-table', 'ping-pong', 'arcade', 'water-cooler']) assert.ok(meta.includes(id), id);
});
