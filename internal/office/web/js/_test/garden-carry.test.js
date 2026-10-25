// agents survive a campus rebuild: movement state carries over and paths re-plan on the new walk graph
import test from 'node:test';
import assert from 'node:assert/strict';
import { fixture } from './garden-fixtures.js';
import { compileGarden } from '../garden/compat.js';
import { assignGardenSeats } from '../garden/slots.js';
import { levelOf, replanPath, slotMoved, snapshotMove, restoreMove, remapAgent, walkToSeat } from '../garden/carry.js';

const mains = (accts) => accts.map((a, i) => ({ id: 'm' + i, account: a, state: 'busy', subs: [] }));
const accts = (n) => Array.from({ length: n }, (_, i) => 'acct' + (i + 1));

// a duck typed agent: just the fields and methods the carry code touches
function fake(slot, over = {}) {
  const a = {
    mode: 'seated', slot, path: null, plan: null, curNode: slot.node, spot: null, curSpot: null, abort: false, L: null,
    model: { position: { x: slot.x, y: slot.y, z: slot.z }, rotation: { y: 0 } },
    cupHeld: false, cupHotUntil: 0, nextBreakAt: 7, breakCount: 2, carryPaper: false, s: { state: 'busy' },
    follow(kind, fn, done, dest) { this.path = { kind, fn, wp: 0, done, dest }; this.mode = 'walking'; },
    detach() {}, place(p, yaw) { this.model.position = { x: p.x, y: p.y, z: p.z }; this.model.rotation.y = yaw; },
    arrive() { this.mode = 'seated'; }, leaveChair() { this.mode = 'walking'; },
  };
  return Object.assign(a, over);
}

test('movement snapshot round trips onto a fresh agent', () => {
  const L = compileGarden(fixture('gen-village-3'));
  const slot = L.seatSlot(L.seats.find((s) => s.kind === 'desk'));
  const a = fake(slot, { mode: 'walking', cupHeld: true, cupHotUntil: 31.5, nextBreakAt: 44, breakCount: 3 });
  a.model.position = { x: 3.25, y: 0.4, z: -8.5 };
  a.model.rotation.y = 1.2;
  const snap = snapshotMove(a);
  const b = fake(slot, { cupHeld: false, nextBreakAt: 0, breakCount: 0 });
  restoreMove(b, snap);
  assert.deepEqual(snapshotMove(b), snap);
  assert.equal(b.cupHeld, true);
  assert.equal(b.breakCount, 3);
  assert.deepEqual(snap.pos, { x: 3.25, y: 0.4, z: -8.5 });
});

test('levelOf splits ground and deck', () => {
  assert.equal(levelOf(0), 0);
  assert.equal(levelOf(1.29), 1);
});

// a path from a point that was mid-walk on the old graph, planned on the graph of a bigger campus
test('re-plan from a mid-path position on a changed walk graph', () => {
  for (const [from, to] of [['gen-village-3', 'gen-village-8'], ['gen-river-3', 'gen-river-8'], ['gen-village-8', 'gen-village-3']]) {
    const A = compileGarden(fixture(from));
    const B = compileGarden(fixture(to));
    const shared = A.seats.filter((s) => s.kind === 'desk' && B.seatById.has(s.id)).slice(0, 6);
    assert.ok(shared.length >= 3, from + ' to ' + to + ' share seats');
    for (const s of shared) {
      const old = A.path(A.spawn, A.seatSlot(s).node);
      const mid = old[Math.floor(old.length / 2)];
      const pos = { x: mid.x + 0.1, y: mid.y, z: mid.z };
      const dest = B.seatSlot(B.seatById.get(s.id)).node;
      const pts = replanPath(B, pos, dest);
      assert.ok(pts.length >= 2, s.id);
      assert.equal(pts[0].x, pos.x);
      assert.equal(pts[0].z, pos.z);
      const end = B.path(B.spawn, dest).at(-1);
      assert.deepEqual([pts.at(-1).x, pts.at(-1).z], [end.x, end.z]);
      for (const p of pts) assert.ok(Number.isFinite(p.x) && Number.isFinite(p.z) && Number.isFinite(p.y));
    }
  }
});

test('slotMoved: same seat is not a move, another seat or kind is', () => {
  const L = compileGarden(fixture('gen-village-3'));
  const [d1, d2] = L.seats.filter((s) => s.kind === 'desk');
  const s1 = L.seatSlot(d1);
  assert.equal(slotMoved(s1, s1), false);
  assert.equal(slotMoved(s1, L.seatSlot(d2)), true);
  assert.equal(slotMoved(s1, L.standSlot(0, 1, 1)), true);
  const st = L.standSlot(0, 1, 1);
  assert.equal(slotMoved(st, L.standSlot(0, 1, 1)), false);
});

test('a seat that vanished sends the agent walking to its new seat', () => {
  const big = compileGarden(fixture('gen-village-8'));
  const small = compileGarden(fixture('gen-village-3'));
  const sessions = mains(accts(8));
  const first = assignGardenSeats(big, sessions, null, null);
  const next = assignGardenSeats(small, sessions, null, first.plan);
  let moved = 0;
  for (const [key, e] of first.plan) {
    if (e.kind !== 'seat' || small.seatById.has(e.seat)) continue;
    const ns = next.slots.get(key);
    const old = first.slots.get(key);
    assert.equal(slotMoved(old, ns), true, key);
    const a = fake(old);
    const snap = snapshotMove(a);
    a.slot = ns;
    walkToSeat(a, snap, small);
    assert.equal(a.mode, 'walking');
    assert.equal(a.path.kind, 'back');
    const pts = a.path.fn();
    assert.equal(pts.length >= 2, true);
    const goal = small.path(small.spawn, ns.node).at(-1);
    assert.deepEqual([pts.at(-1).x, pts.at(-1).z], [goal.x, goal.z]);
    a.path.done();
    assert.equal(a.mode, 'seated');
    moved++;
  }
  assert.ok(moved > 0, 'some seats vanished going from 8 to 3 accounts');
});

test('remap: a walking agent re-plans toward the same goal on the new graph', () => {
  const A = compileGarden(fixture('gen-village-3'));
  const B = compileGarden(fixture('gen-village-8'));
  const seat = A.seats.find((s) => s.kind === 'desk' && B.seatById.has(s.id));
  const slotA = A.seatSlot(seat);
  const slotB = B.seatSlot(B.seatById.get(seat.id));
  const old = A.path(A.spawn, slotA.node);
  const a = fake(slotA, { mode: 'walking' });
  a.model.position = { x: old[3].x, y: 0, z: old[3].z };
  a.path = { kind: 'in', fn: () => A.path(A.spawn, slotA.node), wp: 3, done: () => {}, dest: () => slotA.node };
  a.path.dest = (L, ag) => ag.slot.node;
  remapAgent(a, B, slotB);
  assert.equal(a.slot, slotB);
  assert.equal(a.path.wp, 0);
  const pts = a.path.fn();
  assert.deepEqual([pts[0].x, pts[0].z], [old[3].x, old[3].z]);
  const goal = B.path(B.spawn, slotB.node).at(-1);
  assert.deepEqual([pts.at(-1).x, pts.at(-1).z], [goal.x, goal.z]);
});

test('remap: an agent leaving keeps leaving, through the lobby of the new graph', () => {
  const A = compileGarden(fixture('gen-village-3'));
  const B = compileGarden(fixture('gen-village-8'));
  const seat = A.seats.find((s) => s.kind === 'desk' && B.seatById.has(s.id));
  const slotA = A.seatSlot(seat);
  const slotB = B.seatSlot(B.seatById.get(seat.id));
  const a = fake(slotA, { mode: 'walking', s: { state: 'ended', leaving: true } });
  const out = A.path(slotA.node, A.spawn);
  a.model.position = { x: out[2].x, y: 0, z: out[2].z };
  a.path = { kind: 'out', fn: () => out, wp: 2, done: () => {}, dest: (L) => L.spawn };
  remapAgent(a, B, slotB);
  assert.equal(a.path.kind, 'out');
  const pts = a.path.fn();
  assert.deepEqual([pts.at(-1).x, pts.at(-1).z], [B.spawn.x, B.spawn.z]);
});

test('remap: a break spot that vanished aborts the break, a kept spot is swapped for the new one', () => {
  const A = compileGarden(fixture('gen-village-3'));
  const B = compileGarden(fixture('gen-village-8'));
  const seat = A.seats.find((s) => s.kind === 'desk' && B.seatById.has(s.id));
  const slotA = A.seatSlot(seat);
  const slotB = B.seatSlot(B.seatById.get(seat.id));
  const spot = Object.values(A.spots)[0];
  const a = fake(slotA, { mode: 'leisure', spot, curSpot: spot.name, curNode: spot.node });
  remapAgent(a, B, slotB);
  assert.equal(a.spot, B.spots[spot.name]);
  assert.equal(a.abort, false);
  const gone = fake(slotA, { mode: 'leisure', spot: { ...spot, name: 'nope' }, curSpot: 'nope', curNode: spot.node });
  remapAgent(gone, B, slotB);
  assert.equal(gone.spot, null);
  assert.equal(gone.abort, true);
});

test('remap: seat-bound agents follow the new scene, walkers keep their place', () => {
  const A = compileGarden(fixture('gen-village-3'));
  const B = compileGarden(fixture('gen-village-8'));
  const seat = A.seats.find((s) => s.kind === 'desk' && B.seatById.has(s.id));
  const slotB = B.seatSlot(B.seatById.get(seat.id));
  const rig = { position: { set(x, y, z) { Object.assign(this, { x, y, z }); } }, rotation: { y: 0 } };
  const st = fake(A.seatSlot(seat), { mode: 'standing', rig });
  remapAgent(st, B, slotB);
  const last = slotB.stand.at(-1);
  assert.deepEqual([rig.position.x, rig.position.z], [slotB.x, slotB.z]);
  assert.deepEqual([st.model.position.x, st.model.position.z], [last.x, last.z]);
  const spot = Object.values(A.spots)[0];
  const lz = fake(A.seatSlot(seat), { mode: 'leisure', spot, curSpot: spot.name, curNode: spot.node });
  remapAgent(lz, B, slotB);
  assert.deepEqual([lz.model.position.x, lz.model.position.z], [B.spots[spot.name].x, B.spots[spot.name].z]);
});
