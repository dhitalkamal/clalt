// the seat pipeline over odd layouts: nothing throws, every agent gets a slot, paths are finite
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { compile } from '../web/js/scene/plan.js';
import { assignSeats } from '../web/js/scene/seats.js';

const read = (p) => JSON.parse(fs.readFileSync(new URL(p, import.meta.url)));
const catalog = read('../layout/data/catalog.json');
const base = read('../layout/data/templates/default.json');

// small seeded random so a failure can be replayed
const rng = (seed) => () => (seed = (seed * 1664525 + 1013904223) % 4294967296) / 4294967296;

function roster(n, m) {
  const sessions = [];
  for (let i = 0; i < n; i++) {
    const s = { id: 's' + i, account: i % 2 ? 'a' : 'b', state: 'busy', subs: [] };
    for (let j = 0; j < m; j++) s.subs.push({ id: s.id + '/' + j, type: ['Explore', 'Plan', 'code-reviewer', 'general-purpose'][j % 4] });
    sessions.push(s);
  }
  return sessions;
}

function randomLayout(r) {
  const lay = structuredClone(base);
  lay.size = { w: 20 + Math.floor(r() * 40), d: 12 + Math.floor(r() * 14) };
  lay.door = { side: ['left', 'right', 'back', 'front'][Math.floor(r() * 4)], at: 2 * Math.floor(r() * 5) };
  if (r() < 0.4) { lay.deck = null; lay.stairs = null; lay.rooms = lay.rooms.filter((x) => x.level === 0); lay.furniture = lay.furniture.filter((f) => f.level === 0); }
  const keep = (x, z) => x < lay.size.w && z < lay.size.d;
  lay.rooms = lay.rooms.filter((x) => r() < 0.8 && keep(x.x + x.w - 1, x.z + x.d - 1));
  lay.furniture = lay.furniture.filter((f) => r() < 0.85 && keep(f.x + 4, f.z + 4));
  for (const f of lay.furniture) if (r() < 0.2) f.rot = [0, 90, 180, 270][Math.floor(r() * 4)];
  return lay;
}

test('random layouts compile and every agent gets a slot with a finite path', () => {
  const r = rng(42);
  for (let k = 0; k < 40; k++) {
    const lay = randomLayout(r);
    const L = compile(lay, catalog);
    const sessions = roster(1 + Math.floor(r() * 9), Math.floor(r() * 5));
    const { slots } = assignSeats(L, sessions, catalog, null);
    const keys = sessions.flatMap((s) => [s.id, ...s.subs.map((x) => x.id)]);
    for (const key of keys) {
      const slot = slots.get(key);
      assert.ok(slot, 'no slot for ' + key + ' in layout ' + k);
      for (const p of L.path(L.spawn, slot.node)) assert.ok(Number.isFinite(p.x + p.z + p.y));
      for (const p of L.path(slot.node, L.spawn)) assert.ok(Number.isFinite(p.x + p.z + p.y));
    }
  }
});

test('agents go to the room their type asks for, then the workspace, then the lobby', () => {
  const L = compile(base, catalog);
  const { slots } = assignSeats(L, roster(1, 4), catalog, null);
  const roomOf = (key) => slots.get(key).room;
  assert.equal(roomOf('s0'), 'cabins');
  assert.equal(roomOf('s0/0'), 'research');
  assert.equal(roomOf('s0/1'), 'meeting');
  assert.equal(roomOf('s0/2'), 'review');
  assert.equal(roomOf('s0/3'), 'open');
  // seven mains: six cabins, the seventh takes an open workspace seat
  const many = assignSeats(L, roster(7, 0), catalog, null).slots;
  assert.equal([...many.values()].filter((s) => s.room === 'cabins').length, 6);
  assert.equal(many.get('s6').room, 'open');
  // far more agents than seats: the rest stand in the lobby
  const crowd = assignSeats(L, roster(14, 0), catalog, null).slots;
  assert.ok([...crowd.values()].some((s) => s.kind === 'stand' && L.rooms.find((x) => x.id === 'lobby')));
});

test('a layout without a review room seats reviewers in the workspace', () => {
  const lay = structuredClone(base);
  lay.rooms = lay.rooms.filter((r) => r.id !== 'review');
  lay.furniture = lay.furniture.filter((f) => !(f.level === 1 && ['table-seat', 'round-table'].includes(f.item)));
  const L = compile(lay, catalog);
  const { slots } = assignSeats(L, roster(1, 3), catalog, null);
  assert.equal(slots.get('s0/2').room, 'open');
});
