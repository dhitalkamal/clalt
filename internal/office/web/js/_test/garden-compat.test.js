// the hall-shaped view of a garden (slots, spots, groups, paths) and the slots built from the routing plan
import test from 'node:test';
import assert from 'node:assert/strict';
import { fixture, ALL } from './garden-fixtures.js';
import { compileGarden } from '../garden/compat.js';
import { assignGardenSeats } from '../garden/slots.js';

const main = (key, account) => ({ key, kind: 'main', account, type: 'main', state: 'busy' });
const sub = (key, account, type) => ({ key, kind: 'sub', account, type, state: 'busy' });

// a flat roster back into the sessions the page keeps (every sub hangs under the first main)
const sessionsOf = (roster) => {
  const mains = roster.filter((a) => a.kind === 'main').map((a) => ({ id: a.key, account: a.account, state: a.state, subs: [] }));
  for (const a of roster.filter((x) => x.kind === 'sub')) (mains.find((m) => m.account === a.account) || mains[0]).subs.push({ id: a.key, type: a.type });
  return mains;
};

test('every seat gets a slot the agent code can use', () => {
  for (const name of ALL) {
    const L = compileGarden(fixture(name));
    for (const s of L.seats) {
      const slot = L.seatSlot(s);
      assert.ok(['desk', 'round'].includes(slot.kind), s.id);
      for (const k of ['x', 'z', 'y', 'rotY']) assert.ok(Number.isFinite(slot[k]), `${s.id} ${k}`);
      assert.ok(slot.node && slot.chair && slot.corr && Array.isArray(slot.stand), s.id);
      assert.equal(slot.room, s.pavilionId);
    }
  }
});

test('a desk slot puts the chair behind the desk and a round slot puts the rig toward the table', () => {
  const L = compileGarden(fixture('rows'));
  const desk = L.seatSlot(L.seats.find((s) => s.kind === 'desk'));
  assert.ok(Math.abs(desk.chair.z - (desk.z - 0.36)) < 1e-9, 'rot 0 chair is north of the desk');
  const rev = L.seats.find((s) => s.kind === 'review');
  const r = L.seatSlot(rev);
  assert.equal(r.kind, 'round');
  assert.ok(Math.abs(Math.hypot(r.x - r.chair.x, r.z - r.chair.z) - 0.36) < 1e-9);
  assert.ok(Math.abs(r.y - 1.29) < 1e-9);
});

test('break spots and groups feed the break planner', () => {
  const L = compileGarden(fixture('river'));
  assert.equal(L.groups.counter.length, 2);
  assert.equal(L.groups.table.length, 4);
  assert.equal(L.groups.pong.length, 2);
  assert.equal(L.groups.bag.length, 3);
  for (const t of L.groups.table) assert.ok(L.spots[t.partner], 'table partner exists');
  for (const sp of Object.values(L.spots)) assert.ok(sp.node && Number.isFinite(sp.yaw));
});

test('paths join the spawn and slot nodes', () => {
  const L = compileGarden(fixture('river'));
  for (const s of L.seats.filter((x) => x.kind === 'desk').slice(0, 4)) {
    const slot = L.seatSlot(s);
    const pts = L.path(L.spawn, slot.node);
    assert.ok(pts.length >= 3);
    const back = L.path(slot.node, L.spawn);
    assert.ok(back.length >= 3);
  }
  const cafe = Object.values(L.spots).find((s) => s.act === 'order');
  const sip = Object.values(L.spots).find((s) => s.act === 'sip');
  assert.ok(L.path(cafe.node, sip.node).length >= 2);
});

test('regions cover the pavilions and split the treehouse', () => {
  const L = compileGarden(fixture('rows'));
  assert.ok(L.regions.some((r) => r.type === 'review' && r.y > 1));
  assert.ok(L.regions.some((r) => r.type === 'meeting' && r.y > 1));
  assert.ok(L.regions.some((r) => r.type === 'lobby' && r.y === 0));
  assert.ok(L.W > 20 && L.D > 10);
});

test('planner: mains take their cabin, subagents their room', () => {
  const L = compileGarden(fixture('village'));
  const roster = [main('m1', 'work'), main('m2', 'personal'), main('m3', 'client'), sub('s1', 'work', 'Explore'), sub('s2', 'work', 'Plan'),
    sub('s3', 'work', 'code-reviewer'), sub('s4', 'work', 'general-purpose')];
  const { slots } = assignGardenSeats(L, sessionsOf(roster), null, null);
  const seat = (k) => L.seatById.get(slots.get(k).itemId);
  assert.equal(seat('m1').account, 'work');
  assert.equal(seat('m2').account, 'personal');
  assert.equal(seat('m3').account, 'client');
  assert.equal(seat('s1').type, 'research');
  assert.equal(seat('s2').kind, 'meeting');
  assert.equal(seat('s3').kind, 'review');
  assert.equal(seat('s4').type, 'workspace');
  assert.equal(new Set([...slots.values()].map((e) => e.itemId)).size, roster.length);
});

test('planner: a full workspace spills into overflow spots, then the lobby; seats stay put', () => {
  const L = compileGarden(fixture('river'));
  const roster = [main('m1', 'work')];
  for (let i = 0; i < 20; i++) roster.push(sub('g' + i, 'work', 'general-purpose'));
  const { plan } = assignGardenSeats(L, sessionsOf(roster), null, null);
  const stand = [...plan.values()].filter((e) => e.kind === 'stand');
  assert.ok(stand.length >= 1 && stand.length < 20, 'every free work desk is used, the rest stands');
  for (const e of stand) assert.ok(Number.isFinite(e.x) && e.world === true);
  // nobody stacks on another standing agent, even past the last spot
  assert.equal(new Set(stand.map((e) => e.x.toFixed(3) + ',' + e.z.toFixed(3))).size, stand.length);
  const again = assignGardenSeats(L, sessionsOf(roster), null, plan).plan;
  assert.deepEqual([...again.entries()], [...plan.entries()]);
  // one agent leaves: the seated ones keep their desks, a standing one may take the free desk
  const fewer = roster.filter((a) => a.key !== 'g0');
  const after = assignGardenSeats(L, sessionsOf(fewer), null, plan).plan;
  for (const [k, e] of after) if (plan.get(k).kind === 'seat') assert.deepEqual(e, plan.get(k));
});

// every walk between a desk and a break spot stays on open ground: no leg crosses water, a trunk or a wall
test('walks between desks, spots and the spawn never cross blocked cells', () => {
  for (const name of ALL) {
    const L = compileGarden(fixture(name));
    const w = L.walk;
    const open = (x, z, y) => {
      const c = { x: Math.floor(w.f.cx(x)), z: Math.floor(w.f.cz(z)) };
      const g = y > 0.5 ? w.grids.deck : w.grids.lawn;
      return c.x >= 0 && c.z >= 0 && c.x < g.w && c.z < g.d && g.open[c.z * g.w + c.x] === 1;
    };
    const nodes = [L.spawn, ...L.seats.filter((s) => s.kind === 'desk').filter((_, i) => i % 3 === 0).map((s) => L.seatSlot(s).node), ...Object.values(L.spots).map((s) => s.node),
      ...L.seats.filter((s) => s.kind === 'review' || s.kind === 'meeting').map((s) => L.seatSlot(s).node)];
    let checked = 0;
    for (const a of nodes) {
      for (const b of nodes) {
        if (a === b) continue;
        const pts = L.path(a, b);
        for (let i = 1; i < pts.length; i++) {
          const p = pts[i - 1];
          const q = pts[i];
          if (p.y !== q.y) continue;
          const len = Math.hypot(q.x - p.x, q.z - p.z);
          // the first and last legs run through desks and chairs, which no grid knows about
          if (i === 1 || i === pts.length - 1 || len < 2.2) continue;
          for (let t = 0.1; t < 0.95; t += 0.1) {
            checked++;
            assert.ok(open(p.x + (q.x - p.x) * t, p.z + (q.z - p.z) * t, p.y), `${name}: leg ${p.x.toFixed(1)},${p.z.toFixed(1)} -> ${q.x.toFixed(1)},${q.z.toFixed(1)} leaves open ground`);
          }
        }
      }
    }
    assert.ok(checked > 0, name);
  }
});
