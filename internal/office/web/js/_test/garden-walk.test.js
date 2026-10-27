// the walk graph: paths, bridges, floors and stairs walkable; water and trees not
import test from 'node:test';
import assert from 'node:assert/strict';
import { fixture, NAMES, ALL } from './garden-fixtures.js';
import { gardenWalk } from '../garden/walk.js';
import { gardenSeats } from '../garden/seats.js';

test('paths and pavilion floors are walkable, water is not', () => {
  const L = fixture('river');
  const w = gardenWalk(L);
  const p = L.site.paths[0].pts[0];
  assert.ok(w.walkable(p[0], p[1], 0), 'path start');
  const lobby = L.pavilions.find((x) => x.type === 'lobby');
  assert.ok(w.walkable(lobby.x, lobby.z + 1, 0) || w.walkable(lobby.x + 1, lobby.z, 0), 'lobby floor');
  const mid = L.site.river.pts[3];
  assert.ok(!w.walkable(mid[0], mid[1], 0), 'river centre');
  const pond = L.site.ponds[0];
  assert.ok(!w.walkable(pond.x, pond.z, 0), 'pond');
});

test('a bridge crosses the river', () => {
  const L = fixture('river');
  const w = gardenWalk(L);
  const b = L.site.bridges[1];
  const mx = (b.a[0] + b.b[0]) / 2;
  const mz = (b.a[1] + b.b[1]) / 2;
  assert.ok(w.walkable(mx, mz, 0));
});

test('every seat is reachable from the gate on roads alone', () => {
  // the cafe sits on the green by design and uses the lawn fallback, every other seat must be reachable
  for (const name of ALL) {
    const L = fixture(name);
    const w = gardenWalk(L);
    const bad = [];
    for (const s of gardenSeats(L).filter((x) => x.kind !== 'cafe')) {
      const n = w.nodeForSeat(s);
      if (!w.reachable(n)) bad.push(s.id);
    }
    assert.deepEqual(bad, [], name + ' unreachable: ' + bad.join(','));
  }
});

test('a seat off the roads still gets a walk over the lawn', () => {
  const L = fixture('village');
  const w = gardenWalk(L);
  const cafe = gardenSeats(L).find((s) => s.kind === 'cafe');
  const pts = w.path(w.spawn, w.nodeForSeat(cafe));
  assert.ok(pts.length >= 3);
  for (let i = 1; i < pts.length; i++) assert.ok(Math.hypot(pts[i].x - pts[i - 1].x, pts[i].z - pts[i - 1].z) < 25);
});

test('paths between nodes start and end at the nodes, with heights on the stairs', () => {
  const L = fixture('rows');
  const w = gardenWalk(L);
  const seats = gardenSeats(L);
  const rev = seats.find((s) => s.kind === 'review');
  const desk = seats.find((s) => s.kind === 'desk');
  const a = w.nodeForSeat(desk);
  const b = w.nodeForSeat(rev);
  const pts = w.path(a, b);
  assert.ok(pts.length > 3);
  assert.ok(Math.hypot(pts[0].x - a.x, pts[0].z - a.z) < 1e-6);
  const last = pts[pts.length - 1];
  assert.ok(Math.hypot(last.x - b.x, last.z - b.z) < 1e-6);
  assert.ok(pts.some((p) => p.y > 1.2), 'goes up to the deck');
  assert.ok(pts.every((p) => Number.isFinite(p.x) && Number.isFinite(p.z) && Number.isFinite(p.y)));
  // no leg is a teleport: consecutive points stay within a few units except the spawn hop
  for (let i = 1; i < pts.length; i++) assert.ok(Math.hypot(pts[i].x - pts[i - 1].x, pts[i].z - pts[i - 1].z) < 25);
});

test('the gate spawn leads to every desk', () => {
  const L = fixture('village');
  const w = gardenWalk(L);
  for (const s of gardenSeats(L).filter((x) => x.kind === 'desk').slice(0, 6)) {
    const pts = w.path(w.spawn, w.nodeForSeat(s));
    assert.ok(pts.length >= 2, s.id);
  }
});

test('walk cells stay off tree trunks', () => {
  const L = fixture('river');
  const w = gardenWalk(L);
  for (const t of w.trees.slice(0, 40)) assert.ok(!w.walkable(t.x, t.z, 0));
});

// pickets stop agents: no walk cell on a picket is open, on roads or on the lawn
test('walk cells stay off the picket fence', async () => {
  const { fenceParts } = await import('../garden/fencepts.js');
  for (const name of NAMES) {
    const L = fixture(name);
    const w = gardenWalk(L);
    const { pickets } = fenceParts(w.site);
    assert.ok(pickets.length > 20, name + ' has a fence');
    const open = (g, x, z) => {
      const cx = Math.floor(w.f.cx(x));
      const cz = Math.floor(w.f.cz(z));
      return cx >= 0 && cz >= 0 && cx < g.w && cz < g.d && g.open[cz * g.w + cx] === 1;
    };
    const bad = pickets.filter((p) => open(w.grids.roads, p.x, p.z) || open(w.grids.lawn, p.x, p.z));
    assert.equal(bad.length, 0, `${name}: ${bad.length} of ${pickets.length} pickets are walkable`);
  }
});

// a deck seat walled off from the landing still gets a walk across the open deck, not a straight cut
test('a cut-off deck seat is bridged from the nearest open deck cell', () => {
  const L = fixture('gen-village-10-16-40');
  const w = gardenWalk(L);
  const seat = gardenSeats(L).find((s) => s.id.startsWith('treehouse') && s.y > 0.5);
  const n = w.nodeForSeat(seat);
  const d = w.grids.deck;
  const c = { x: Math.floor(w.f.cx(n.ap.x)), z: Math.floor(w.f.cz(n.ap.z)) };
  // a closed ring two cells out isolates the seat from the rest of the deck
  for (let dz = -2; dz <= 2; dz++) for (let dx = -2; dx <= 2; dx++) {
    if (Math.max(Math.abs(dx), Math.abs(dz)) === 2) d.open[(c.z + dz) * d.w + c.x + dx] = 0;
  }
  const pts = w.path(w.spawn, n);
  const onDeck = pts.filter((p) => p.y > 1.2);
  // the last hop is the short jump over the ring, not a cut across the whole deck
  const [p, q] = onDeck.slice(-2);
  const land = onDeck[onDeck.length - 4];
  assert.ok(Math.hypot(q.x - p.x, q.z - p.z) < 0.6 * Math.hypot(q.x - land.x, q.z - land.z), 'ends beside the seat');
});
