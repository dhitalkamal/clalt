// the hall adapter: walkable grid, spawn, bounds, stairs descriptor and presets on the embedded default layout
import test from 'node:test';
import assert from 'node:assert/strict';
import { hallCompiled } from './hall-fixture.js';
import { hallCtx } from '../nav/hallctx.js';
import { stairDescOf, stairFrame } from '../nav/groundmath.js';
import { stairsOf } from '../garden/walkgrid.js';
import { gardenWalk } from '../garden/walk.js';
import { fixture } from './garden-fixtures.js';
import { solidRect } from '../scene/itemmeta.js';
import { solidCells } from '../scene/geom.js';
import { presetPose, project } from '../garden/cameras.js';

const L = hallCompiled();
const h = hallCtx(L);

test('walls and the room edge block, the floor and the door cells are open', () => {
  assert.equal(h.walk.walkable(L.xmin - 0.3, 0, 0), false);
  assert.equal(h.walk.walkable(L.xmin + L.W + 0.3, 0, 0), false);
  assert.equal(h.walk.walkable(0, L.zmin - 0.3, 0), false);
  let open = 0;
  for (let i = 0; i < L.layout.size.w; i++) for (let j = 0; j < L.layout.size.d; j++) if (h.walk.walkable(L.X(i + 0.5), L.Z(j + 0.5), 0)) open++;
  assert.ok(open > 0.5 * L.layout.size.w * L.layout.size.d, 'most of the floor is open: ' + open);
  // the door sits on the left wall: its cells are open and so is the spot just inside
  const d = L.door;
  assert.equal(h.walk.walkable(d.th.x + 0.25, d.th.z, 0), true);
});

test('every solid furniture cell is blocked', () => {
  let n = 0;
  for (const it of L.items) {
    const rect = solidRect(it.def);
    if (!rect) continue;
    for (const c of solidCells(rect, it.def.footprint, it.rot)) {
      const x = L.X(it.rec.x + c.i + 0.5);
      const z = L.Z(it.rec.z + c.j + 0.5);
      assert.equal(h.walk.walkable(x, z, it.level), false, it.id + ' cell ' + c.i + ',' + c.j);
      n++;
    }
  }
  assert.ok(n > 50);
});

test('the stair flight is closed on the ground and the deck is open on level 1', () => {
  const s = h.stairs;
  const mid = { x: s.foot.x + s.dir[0] * s.len / 2, z: s.foot.z + s.dir[1] * s.len / 2 };
  assert.equal(h.walk.walkable(mid.x, mid.z, 0), false);
  const dk = L.deck;
  assert.equal(h.walk.walkable((dk.x0 + dk.x1) / 2, (dk.z0 + dk.z1) / 2, 1), true);
  assert.equal(h.walk.walkable(L.xmin + 2, L.zmin + 12, 1), false);
});

test('spawn is at the door and walkable', () => {
  const ap = h.walk.spawn.ap;
  assert.ok(h.walk.walkable(ap.x, ap.z, 0));
  assert.ok(Math.abs(ap.x - L.xmin) < 1);
});

test('bounds and bb come from the hall size', () => {
  assert.deepEqual(h.bounds, { minX: L.xmin, maxX: L.xmin + L.W, minZ: L.zmin, maxZ: L.zmin + L.D });
  assert.equal(h.bb.x0, L.xmin);
  assert.equal(h.bb.z1, L.zmin + L.D);
  assert.equal(h.groundY(0, 0), 0);
  assert.ok(Math.abs(h.groundY(0, 0, 1) - 1.29) < 1e-9);
});

// flood over cell centres: the walker's disc is tight in one-cell lanes, so only the centre counts
function flood(level, seed) {
  const seen = new Set();
  const key = (x, z) => x + ',' + z;
  const st = [seed];
  seen.add(key(seed.x, seed.z));
  while (st.length) {
    const c = st.pop();
    for (const [dx, dz] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
      const n = { x: c.x + dx, z: c.z + dz };
      if (seen.has(key(n.x, n.z)) || !h.walk.walkable(L.X(n.x + 0.5), L.Z(n.z + 0.5), level)) continue;
      seen.add(key(n.x, n.z));
      st.push(n);
    }
  }
  return seen;
}

test('every seat is reachable from the spawn, deck seats through the stairs', () => {
  const ap = h.walk.spawn.ap;
  const ground = flood(0, L.cellOf(ap));
  const s = h.stairs;
  const before = L.cellOf({ x: s.foot.x - s.dir[0] * 0.3, z: s.foot.z - s.dir[1] * 0.3 });
  assert.ok(ground.has(before.x + ',' + before.z), 'the foot of the stairs is reachable');
  const top = flood(1, L.cellOf({ x: s.foot.x + s.dir[0] * (s.len + 0.6), z: s.foot.z + s.dir[1] * (s.len + 0.6) }));
  let seats = 0;
  for (const it of L.items) {
    if (!it.meta || !it.meta.seat) continue;
    const slot = L.seatSlot(it);
    const c = L.cellOf(slot.node.ap);
    const set = slot.level === 1 ? top : ground;
    assert.ok(set.has(c.x + ',' + c.z), it.id + ' level ' + slot.level + ' cell ' + c.x + ',' + c.z);
    seats++;
  }
  assert.ok(seats >= 20);
});

test('stairs descriptor of the hall: foot, direction, length, width, level and height', () => {
  const s = h.stairs;
  assert.ok(Math.abs(s.len - 3) < 1e-9);
  assert.equal(s.width, 1);
  assert.equal(s.topLevel, 1);
  assert.ok(Math.abs(s.height - 1.29) < 1e-9);
  assert.deepEqual(s.dir, [0, -1]);
  assert.ok(Math.abs(s.foot.z - L.stairs.B.z) < 1e-9);
  const fr = stairFrame(s);
  assert.equal(fr.len, 3);
  assert.equal(fr.rise, s.height);
});

test('stairs descriptor of the garden matches the old treehouse stairs', () => {
  const walk = gardenWalk(fixture('gen-village-3'));
  const st = stairsOf(walk.site);
  const d = stairDescOf(st);
  assert.deepEqual(d.foot, st.B);
  assert.deepEqual(d.dir, st.dir);
  assert.ok(Math.abs(d.len - Math.hypot(st.T.x - st.B.x, st.T.z - st.B.z)) < 1e-9);
  assert.equal(d.height, st.rise);
  assert.equal(d.topLevel, 1);
  assert.equal(d.width, 1);
  assert.deepEqual(stairFrame(st), stairFrame(d));
});

test('presets fit the hall floor inside the screen minus the safe margins', () => {
  const safe = { left: 0.2, bottom: 0.08, top: 0.02, right: 0 };
  for (const id of ['top', 'plan', 'iso', 'hero']) {
    const pose = presetPose(id, { bb: h.bb, aspect: 16 / 9, safe });
    for (const x of [h.bb.x0, h.bb.x1]) for (const z of [h.bb.z0, h.bb.z1]) {
      const p = project(pose, { x, y: 0, z }, 16 / 9);
      assert.ok(p.x >= -1 + 2 * safe.left - 0.01 && p.x <= 1.001, id + ' x ' + p.x);
      assert.ok(p.y >= -1 + 2 * safe.bottom - 0.01 && p.y <= 1 - 2 * safe.top + 0.01, id + ' y ' + p.y);
    }
  }
});

test('the hall ctx is cached per compiled layout', () => {
  assert.equal(hallCtx(L), h);
});
