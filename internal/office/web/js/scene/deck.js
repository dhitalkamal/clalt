// the upper level: slab, fascia, posts, rails and the staircase of a layout that has a deck
import * as THREE from 'three';
import { box } from '../details.js';
import { MEZ } from './geom.js';

const near = (a, b) => Math.abs(a - b) < 1e-6;

function rail(g, x0, z0, x1, z1) {
  const len = Math.hypot(x1 - x0, z1 - z0);
  if (len < 0.05) return;
  const r = box(len, 0.03, 0.03, 0x8a949c);
  r.position.set((x0 + x1) / 2, MEZ + 0.34, (z0 + z1) / 2);
  r.rotation.y = -Math.atan2(z1 - z0, x1 - x0);
  g.add(r);
  const n = Math.max(2, Math.round(len / 0.25));
  for (let i = 0; i <= n; i++) {
    const bb = box(0.012, 0.34, 0.012, 0x8a949c);
    bb.position.set(x0 + ((x1 - x0) * i) / n, MEZ + 0.17, z0 + ((z1 - z0) * i) / n);
    g.add(bb);
  }
}

// the part of an edge a landing next to it leaves open: [from, to] along the edge or null
function opening(edgeAxis, edgeAt, land, d) {
  const touch = edgeAxis === 'x' ? (near(land.x0, edgeAt) || near(land.x1, edgeAt)) : (near(land.z0, edgeAt) || near(land.z1, edgeAt));
  if (!touch) return null;
  const inDeck = edgeAxis === 'x' ? land.z1 > d.z0 && land.z0 < d.z1 : land.x1 > d.x0 && land.x0 < d.x1;
  return inDeck ? (edgeAxis === 'x' ? [land.z0, land.z1] : [land.x0, land.x1]) : null;
}

// a rail along one edge with the landing's opening cut out
function edgeRail(g, d, land, kind) {
  const spec = {
    s: { axis: 'z', at: d.z1, from: d.x0, to: d.x1 }, n: { axis: 'z', at: d.z0, from: d.x0, to: d.x1 },
    w: { axis: 'x', at: d.x0, from: d.z0, to: d.z1 }, e: { axis: 'x', at: d.x1, from: d.z0, to: d.z1 },
  }[kind];
  const cut = land ? opening(spec.axis, spec.at, land, d) : null;
  const seg = (a, b) => (spec.axis === 'z' ? rail(g, a, spec.at, b, spec.at) : rail(g, spec.at, a, spec.at, b));
  if (!cut) { seg(spec.from, spec.to); return; }
  seg(spec.from, Math.max(spec.from, cut[0]));
  seg(Math.min(spec.to, cut[1]), spec.to);
}

function stairs(g, S, f) {
  const run = 3.0;
  const n = 8;
  const sg = new THREE.Group();
  sg.position.set(S.T.x, 0, S.T.z);
  sg.rotation.y = S.yaw;
  // local frame: the top edge is z = 0, the flight descends toward +z, the landing is at -1..0
  for (let i = 0; i < n; i++) {
    const t = (i + 1) / n;
    const step = box(1.0, 0.04, run / n, 0xd8c9ac);
    step.position.set(0, MEZ * t - 0.02, run - (run * (i + 0.5)) / n);
    sg.add(step);
  }
  const landing = box(1.0, 0.1, 1.0, 0xcaa06c);
  landing.position.set(0, MEZ - 0.05, -0.5);
  sg.add(landing);
  for (const dx of [-0.5, 0.5]) {
    const len = Math.hypot(run, MEZ);
    const str = box(0.03, 0.04, len, 0x8a949c);
    str.position.set(dx, MEZ / 2 + 0.3, run / 2);
    str.rotation.x = Math.atan2(MEZ, run);
    sg.add(str);
    for (let i = 0; i <= 4; i++) {
      const bb = box(0.012, 0.3, 0.012, 0x8a949c);
      bb.position.set(dx, MEZ * (1 - i / 4) + 0.15, (run * i) / 4);
      sg.add(bb);
    }
  }
  g.add(sg);
  void f;
}

export function buildDeck(L) {
  const g = new THREE.Group();
  const d = L.deck;
  if (!d) return g;
  const w = d.x1 - d.x0;
  const dep = d.z1 - d.z0;
  const slab = box(w, 0.1, dep, 0xcaa06c);
  slab.position.set((d.x0 + d.x1) / 2, MEZ - 0.05, (d.z0 + d.z1) / 2);
  slab.receiveShadow = true;
  g.add(slab);
  const fascia = box(w, 0.06, 0.03, 0xf6f1e8);
  fascia.position.set((d.x0 + d.x1) / 2, MEZ - 0.13, d.z1);
  g.add(fascia);
  // posts every 2 units under the front edge
  for (let x = d.x0; x <= d.x1 + 1e-6; x += 2.0) {
    const p = box(0.07, MEZ - 0.1, 0.07, 0xe9e1d2);
    p.position.set(x, (MEZ - 0.1) / 2, d.z1);
    g.add(p);
  }
  const S = L.stairs;
  const land = S && { x0: L.X(S.landing.x), x1: L.X(S.landing.x + S.landing.w), z0: L.Z(S.landing.z), z1: L.Z(S.landing.z + S.landing.d) };
  // no rail against the back wall
  for (const k of ['s', 'w', 'e', ...(d.z0 - L.zmin > 1.0 ? ['n'] : [])]) edgeRail(g, d, land, k);
  if (S) stairs(g, S, L);
  return g;
}
