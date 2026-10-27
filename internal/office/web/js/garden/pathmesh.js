// flagstone paths and wooden bridges
import * as THREE from 'three';
import { std, box, cyl, track, cached, at } from './res.js';
import { flagTex } from './tex.js';
import { DROP } from './terrain.js';

// flat strip along a polyline at height y, uv.x across, uv.y along in world units
export function ribbon(pts, w, y) {
  const pos = [];
  const uv = [];
  const idx = [];
  let run = 0;
  for (let i = 0; i < pts.length; i++) {
    const a = pts[Math.max(0, i - 1)];
    const b = pts[Math.min(pts.length - 1, i + 1)];
    let tx = b[0] - a[0];
    let tz = b[1] - a[1];
    const l = Math.hypot(tx, tz) || 1;
    tx /= l; tz /= l;
    if (i > 0) run += Math.hypot(pts[i][0] - pts[i - 1][0], pts[i][1] - pts[i - 1][1]);
    // mitre scale at corners, capped
    let k = 1;
    if (i > 0 && i < pts.length - 1) {
      const px = pts[i][0] - pts[i - 1][0];
      const pz = pts[i][1] - pts[i - 1][1];
      const pl = Math.hypot(px, pz) || 1;
      k = Math.min(2, 1 / Math.max(0.5, (px / pl) * tx + (pz / pl) * tz));
    }
    pos.push(pts[i][0] + tz * (w / 2) * k, y, pts[i][1] - tx * (w / 2) * k, pts[i][0] - tz * (w / 2) * k, y, pts[i][1] + tx * (w / 2) * k);
    uv.push(0, run, 1, run);
    if (i + 1 < pts.length) { const o = i * 2; idx.push(o, o + 1, o + 2, o + 1, o + 3, o + 2); }
  }
  const g = track(new THREE.BufferGeometry());
  g.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
  g.setAttribute('uv', new THREE.Float32BufferAttribute(uv, 2));
  g.setIndex(idx);
  g.computeVertexNormals();
  return g;
}

// sandy strip with a stone ribbon on top for each path of the site. paths is the smoothed list from site.js
export function buildPaths(paths) {
  const g = new THREE.Group();
  const stone = cached('m:flag', () => new THREE.MeshStandardMaterial({ map: flagTex(2, 2), roughness: 0.95 }));
  const sand = std(0xd8c49a);
  const y = -DROP + 0.012;
  for (const p of paths) {
    const base = new THREE.Mesh(ribbon(p.pts, p.w + 0.22, y - 0.004), sand);
    const top = new THREE.Mesh(ribbon(p.pts, p.w, y), stone);
    base.receiveShadow = true;
    top.receiveShadow = true;
    g.add(base, top);
  }
  return g;
}

// plank bridge with rails between two points, arching a little over the water
export function bridge(a, b, w = 1.1, arch = 0.35) {
  const len = Math.hypot(b[0] - a[0], b[1] - a[1]);
  const n = Math.max(4, Math.round(len / 0.32));
  const g = new THREE.Group();
  const yAt = (t) => Math.sin(Math.PI * t) * arch;
  const inner = new THREE.Group();
  inner.position.set(a[0], 0, a[1]);
  inner.rotation.y = Math.atan2(b[0] - a[0], b[1] - a[1]);
  g.add(inner);
  const tones = [0xb98a5e, 0xa97c52];
  for (let i = 0; i < n; i++) {
    const t0 = i / n;
    const t1 = (i + 1) / n;
    const z0 = len * t0;
    const z1 = len * t1;
    const dy = yAt(t1) - yAt(t0);
    const sl = Math.hypot(dy, z1 - z0);
    const seg = box(w, 0.07, sl + 0.01, tones[i % 2]);
    seg.position.set(0, (yAt(t0) + yAt(t1)) / 2 - 0.035, (z0 + z1) / 2);
    seg.rotation.x = -Math.atan2(dy, z1 - z0);
    inner.add(seg);
    for (const s of [-1, 1]) {
      const r = box(0.035, 0.035, sl + 0.01, 0xf3ead8);
      r.position.set(s * (w / 2 - 0.02), (yAt(t0) + yAt(t1)) / 2 + 0.34, (z0 + z1) / 2);
      r.rotation.x = -Math.atan2(dy, z1 - z0);
      inner.add(r);
    }
  }
  const np = Math.max(2, Math.round(len / 1.2));
  for (let i = 0; i <= np; i++) {
    const t = i / np;
    for (const s of [-1, 1]) inner.add(at(cyl(0.03, 0.035, 0.38, 0x7a5232, 6), s * (w / 2 - 0.02), yAt(t) + 0.17, len * t));
  }
  // piles under the deck so it does not float on water
  for (const t of [0.04, 0.96]) for (const s of [-1, 1]) inner.add(at(cyl(0.05, 0.05, 0.6, 0x5c4330, 6), s * (w / 2 - 0.02), -0.2, len * t));
  return g;
}
