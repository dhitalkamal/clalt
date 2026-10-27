// the walk graph of a garden layout. agents walk on paths, bridges, pavilion floors and the treehouse stairs;
// water, trees, furniture and pavilion walls stop them. when the roads alone do not connect two places the
// router falls back to the open lawn, so a layout with a gap in its paths still works.
// nodes are {id, x, z, y, level, ap, inner}: ap is where the node meets open ground, inner leads from there to
// the exact spot (a chair behind a desk). path(a, b) returns points {x, z, y}, both nodes included.
// pure, no three.js.
import { findPath, isOpen, nearestOpen } from '../scene/nav.js';
import { buildSite } from './site.js';
import { plantPlan } from './plan.js';
import { buildGrids, stairsOf, CELL } from './walkgrid.js';
import { toWorld } from './footprints.js';
import { SEAT_OFF, MEZ, lobbyGate } from './local.js';

const LEVELS = [0, MEZ];
let nextId = 1;

export function gardenWalk(layout) {
  const site = buildSite(layout);
  const plan = plantPlan(layout, site);
  const { f, roads, lawn, deck } = buildGrids(layout, site, plan);
  const st = stairsOf(site);
  const cellOf = (p) => ({ x: Math.floor(f.cx(p.x)), z: Math.floor(f.cz(p.z)) });
  const centre = (c) => ({ x: f.X(c.x + 0.5), z: f.Z(c.z + 0.5) });
  const gridsAt = (level) => (level === 1 ? [deck] : [roads, lawn]);
  const pt = (p, level) => ({ x: p.x, z: p.z, y: LEVELS[level] });

  const walkable = (x, z, level = 0) => {
    const c = cellOf({ x, z });
    return isOpen(level === 1 ? deck : roads, c.x, c.z);
  };

  function node(level, ap, inner = []) {
    const last = inner.length ? inner[inner.length - 1] : ap;
    return { id: nextId++, x: last.x, z: last.z, y: LEVELS[level], level, ap, inner };
  }

  // stairs: entry cell at the foot (a little south of the first step), the landing cell on the deck
  let stairs = null;
  if (st) {
    const k = 0.35;
    const entry = cellOf({ x: st.B.x - st.dir[0] * k, z: st.B.z - st.dir[1] * k });
    stairs = { entry, land: cellOf(st.landing), up: [pt(st.B, 0), pt(st.T, 1), pt(st.landing, 1)] };
  }

  // the cells of a node on each grid it could use
  const cellIn = (g, p) => nearestOpen(g, cellOf(p), 3);

  function walkCells(level, g, from, to) {
    const cells = findPath(g, from, to);
    return cells && cells.map((c) => ({ ...centre({ x: c.x - 0.5, z: c.z - 0.5 }), y: LEVELS[level] }));
  }

  // a walk between two points on one level, roads first
  function walkOn(level, a, b) {
    for (const g of gridsAt(level)) {
      const from = cellIn(g, a);
      const to = cellIn(g, b);
      if (!from || !to) continue;
      const w = walkCells(level, g, from, to);
      if (w) return w;
    }
    return level === 1 ? bridgeDeck(a, b) : null;
  }

  // the deck has no lawn fallback: when a seat is cut off, walk as far as the open deck goes toward it
  function bridgeDeck(a, b) {
    const from = cellIn(deck, a);
    const to = cellIn(deck, b);
    if (!from) return null;
    const seen = new Set([from.x + ',' + from.z]);
    const stack = [from];
    let best = from;
    const gap = (c) => Math.hypot(c.x - (to || cellOf(b)).x, c.z - (to || cellOf(b)).z);
    while (stack.length) {
      const c = stack.pop();
      if (gap(c) < gap(best)) best = c;
      for (const [dx, dz] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
        const n = { x: c.x + dx, z: c.z + dz };
        if (!isOpen(deck, n.x, n.z) || seen.has(n.x + ',' + n.z)) continue;
        seen.add(n.x + ',' + n.z);
        stack.push(n);
      }
    }
    return walkCells(1, deck, from, best);
  }

  function route(a, b) {
    const out = [pt(a, a.level)];
    for (let i = a.inner.length - 2; i >= 0; i--) out.push(pt(a.inner[i], a.level));
    if (a.inner.length) out.push(pt(a.ap, a.level));
    const middle = [];
    if (a.level === b.level) {
      const w = walkOn(a.level, a.ap, b.ap);
      if (w) middle.push(...w.slice(1, -1));
    } else if (st) {
      const up = a.level === 0;
      const foot = centre(stairs.entry);
      const land = centre(stairs.land);
      const w1 = up ? walkOn(0, a.ap, foot) : walkOn(1, a.ap, land);
      const w2 = up ? walkOn(1, land, b.ap) : walkOn(0, foot, b.ap);
      if (w1) middle.push(...w1.slice(1));
      middle.push(...(up ? stairs.up : [...stairs.up].reverse()));
      if (w2) middle.push(...w2.slice(0, -1));
    }
    out.push(...middle);
    out.push(pt(b.ap, b.level));
    for (const p of b.inner) out.push(pt(p, b.level));
    // drop repeats so an agent never waits on a zero length leg
    return out.filter((p, i) => i === 0 || Math.hypot(p.x - out[i - 1].x, p.z - out[i - 1].z) > 1e-3 || Math.abs(p.y - out[i - 1].y) > 1e-3);
  }

  const cache = new Map();
  function path(a, b) {
    const key = a.id + '>' + b.id;
    let r = cache.get(key);
    if (!r) { r = route(a, b); cache.set(key, r); }
    return r;
  }

  // the gate: arrivals appear under the lobby arch and walk in
  const lobby = site.pavs.find((x) => x.p.type === 'lobby');
  let spawn;
  if (lobby) {
    const g = lobbyGate(lobby.p);
    const w = (l) => { const [x, z] = toWorld(lobby.p, l[0], l[1]); return { x, z }; };
    spawn = node(0, w(g.inside), [w(g.arch), w([g.arch[0], g.arch[1] - 0.9])]);
  } else {
    spawn = node(0, { x: (site.bounds.x0 + site.bounds.x1) / 2, z: (site.bounds.z0 + site.bounds.z1) / 2 }, []);
  }

  // node of a seat from seats.js
  function nodeForSeat(s) {
    const level = s.y > 0.5 ? 1 : 0;
    const p = { x: s.x, z: s.z, rot: s.rot };
    if (s.kind === 'desk') {
      const laneX = s.lane * (s.cols > 1 ? 1.05 : 0.85);
      const at = (lx, lz) => { const [x, z] = toWorld({ x: s.x, z: s.z, rot: s.rot }, lx, lz); return { x, z }; };
      return node(0, at(laneX, 0.75), [at(laneX, 0.25), at(laneX, -SEAT_OFF), at(0, -SEAT_OFF)]);
    }
    return node(level, p, []);
  }

  // which cells the gate reaches on roads alone: a flood over the roads grid, up the stairs and over the deck
  let reach = null;
  function flood() {
    const run = (g, start, seen) => {
      const stack = [start];
      seen.add(start.x + ',' + start.z);
      while (stack.length) {
        const c = stack.pop();
        for (let dx = -1; dx <= 1; dx++) {
          for (let dz = -1; dz <= 1; dz++) {
            const x = c.x + dx;
            const z = c.z + dz;
            if ((!dx && !dz) || !isOpen(g, x, z) || seen.has(x + ',' + z)) continue;
            if (dx && dz && (!isOpen(g, c.x + dx, c.z) || !isOpen(g, c.x, c.z + dz))) continue;
            seen.add(x + ',' + z);
            stack.push({ x, z });
          }
        }
      }
    };
    const ground = new Set();
    const up = new Set();
    const start = cellIn(roads, spawn.ap);
    if (start) run(roads, start, ground);
    // a stair foot off the roads is still reached over the lawn, which is what route() does too
    if (stairs && !ground.has(stairs.entry.x + ',' + stairs.entry.z) && isOpen(lawn, stairs.entry.x, stairs.entry.z)) {
      const onLawn = new Set();
      run(lawn, stairs.entry, onLawn);
      if ([...onLawn].some((k) => ground.has(k))) ground.add(stairs.entry.x + ',' + stairs.entry.z);
    }
    if (stairs && ground.has(stairs.entry.x + ',' + stairs.entry.z) && isOpen(deck, stairs.land.x, stairs.land.z)) run(deck, stairs.land, up);
    return { ground, up };
  }

  // roads only: can the node be reached on paths, bridges and floors alone
  function reachable(n) {
    reach = reach || flood();
    const g = n.level === 1 ? deck : roads;
    const c = cellIn(g, n.ap);
    return !!c && (n.level === 1 ? reach.up : reach.ground).has(c.x + ',' + c.z);
  }

  return { cell: CELL, f, site, trees: plan.trees, plan, grids: { roads, lawn, deck }, stairs, spawn, node, nodeForSeat, path, walkable, reachable };
}
