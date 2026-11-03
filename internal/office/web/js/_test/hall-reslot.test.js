// a hall agent moved to another seat walks there over the floor instead of gliding through the furniture
import test from 'node:test';
import assert from 'node:assert/strict';
import { hallCompiled } from './hall-fixture.js';
import { isOpen } from '../scene/nav.js';
import { reslot } from '../reslot.js';
import { tempNode } from '../garden/carry.js';

// a duck typed agent: the fields and methods the carry code touches
function fake(slot, L, over = {}) {
  const a = {
    mode: 'seated', slot, L, path: null, plan: null, planOffer: null, curNode: slot.node, spot: null, s: { state: 'busy' },
    model: { position: { x: slot.x, y: slot.y, z: slot.z }, rotation: { y: 0 } },
    rig: { position: { x: slot.x, y: slot.y, z: slot.z }, rotation: { y: 0 }, scale: { setScalar() {} } },
    cupHeld: false, cupHotUntil: 0, nextBreakAt: 7, breakCount: 0, carryPaper: false,
    follow(kind, fn, done, dest) { this.path = { kind, fn, wp: 0, done, dest }; this.mode = 'walking'; },
    detach() {}, place(p, yaw) { this.model.position = { x: p.x, y: p.y, z: p.z }; this.model.rotation.y = yaw; },
    arrive() { this.mode = 'seated'; },
  };
  return Object.assign(a, over);
}

function deps(L, newSlot) {
  const log = { removed: [], made: [] };
  return {
    log,
    recordOf: (id) => ({ id, kind: 'main', state: 'busy' }),
    removeAgent: (id) => log.removed.push(id),
    makeAgent: (rec) => { const n = fake(newSlot, L); log.made.push(n); return n; },
  };
}

// the cells of an item's footprint, so the start and end seats may be inside furniture
const cellsOf = (it) => {
  const s = new Set();
  for (let i = 0; i < it.fw; i++) for (let j = 0; j < it.fd; j++) s.add((it.rec.x + i) + ',' + (it.rec.z + j));
  return s;
};

const desks = (L) => L.items.filter((it) => it.def.id === 'desk' && it.level === 0 && L.seatSlot(it));
const cabins = (L) => L.items.filter((it) => it.def.id === 'cabin' && it.level === 0 && L.seatSlot(it));

// every leg of a walk stays on open floor, apart from the cells of the seats at either end
function assertClear(L, pts, from, to) {
  const skip = new Set([...cellsOf(from), ...cellsOf(to)]);
  for (let i = 0; i + 1 < pts.length; i++) {
    const [p, q] = [pts[i], pts[i + 1]];
    const n = Math.ceil(Math.hypot(q.x - p.x, q.z - p.z) / 0.05);
    for (let k = 1; k < n; k++) {
      const c = L.cellOf({ x: p.x + (q.x - p.x) * k / n, z: p.z + (q.z - p.z) * k / n });
      assert.ok(isOpen(L.grids[0], c.x, c.z) || skip.has(c.x + ',' + c.z), from.id + ' -> ' + to.id + ' crosses ' + c.x + ',' + c.z);
    }
  }
}

test('a seat moved to another desk of the same kind gets a fresh agent that walks from the old chair', () => {
  const L = hallCompiled();
  const [from, to] = [desks(L)[0], desks(L).at(-1)];
  const a = fake(L.seatSlot(from), L);
  const d = deps(L, L.seatSlot(to));
  reslot(L, 'm1', a, L.seatSlot(to), d);
  assert.deepEqual(d.log.removed, ['m1']);
  assert.equal(d.log.made.length, 1);
  const n = d.log.made[0];
  assert.equal(n.mode, 'walking');
  const pts = n.path.fn();
  const chair = L.seatSlot(from).chair;
  assert.ok(Math.hypot(pts[0].x - chair.x, pts[0].z - chair.z) < 1e-6, 'starts at the old chair');
  const goal = L.seatSlot(to).node;
  const end = pts.at(-1);
  assert.ok(Math.hypot(end.x - goal.x, end.z - goal.z) < 1e-6, 'ends at the new seat');
});

test('the walk to the new seat never crosses other furniture', () => {
  const L = hallCompiled();
  const d1 = desks(L);
  const c1 = cabins(L);
  const pairs = [[d1[0], d1.at(-1)], [d1.at(-1), d1[1]], [d1[2], d1[5]], [c1[0], c1.at(-1)], [c1.at(-1), c1[1]], [c1[2], d1[3]], [d1[4], c1[3]]];
  for (const [from, to] of pairs) {
    const d = deps(L, L.seatSlot(to));
    reslot(L, 'm', fake(L.seatSlot(from), L), L.seatSlot(to), d);
    assertClear(L, d.log.made[0].path.fn(), from, to);
  }
});

test('a walk from the middle of the floor never crosses furniture either', () => {
  const L = hallCompiled();
  const d1 = desks(L);
  const c1 = cabins(L);
  for (const [from, to] of [[c1[0], c1.at(-1)], [d1[0], c1[2]]]) {
    // somewhere along the old route, as when the plan changes while an agent walks
    const old = L.path(L.seatSlot(from).node, L.spawn);
    const mid = old[Math.floor(old.length / 2)];
    const a = fake(L.seatSlot(from), L, { mode: 'walking' });
    a.model.position = { x: mid.x, y: 0, z: mid.z };
    const d = deps(L, L.seatSlot(to));
    reslot(L, 'm', a, L.seatSlot(to), d);
    assertClear(L, d.log.made[0].path.fn(), from, to);
  }
});

test('a route from a point inside furniture leaves by its nearest open cell, not in a straight line', () => {
  const L = hallCompiled();
  const c1 = cabins(L);
  for (const [from, to] of [[c1[0], c1.at(-1)], [c1.at(-1), c1[0]]]) {
    const chair = L.seatSlot(from).chair;
    const pts = L.path(tempNode(L, { x: chair.x, y: 0, z: chair.z }), L.seatSlot(to).node);
    assertClear(L, pts, from, to);
  }
});

test('a seated agent leaves its old seat the way it came in, through its own door', () => {
  const L = hallCompiled();
  const c1 = cabins(L);
  const d1 = desks(L);
  for (const [from, to] of [[c1[0], c1.at(-1)], [d1[0], c1[2]]]) {
    const d = deps(L, L.seatSlot(to));
    reslot(L, 'm', fake(L.seatSlot(from), L), L.seatSlot(to), d);
    const want = L.path(L.seatSlot(from).node, L.seatSlot(to).node);
    assert.deepEqual(d.log.made[0].path.fn(), want);
  }
});

test('an agent whose seat did not change is kept as it is', () => {
  const L = hallCompiled();
  const slot = L.seatSlot(desks(L)[0]);
  const a = fake(slot, L);
  const d = deps(L, slot);
  reslot(L, 'm1', a, slot, d);
  assert.deepEqual(d.log.removed, []);
  assert.equal(d.log.made.length, 0);
  assert.equal(a.mode, 'seated');
});
