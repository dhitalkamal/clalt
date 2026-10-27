// trees, bushes and lawn tufts from the planting plan, drawn as a handful of instanced meshes: one for trunks,
// one for round foliage, one for pine cones, one for tufts. a few hundred trees cost a few draw calls.
import * as THREE from 'three';
import { rng } from './geo.js';
import { flat, geo, track } from './res.js';
import { DROP } from './terrain.js';

const LEAF = [0x6fb45a, 0x82c264, 0x5aa24f, 0x93cb6a, 0x78b85f];
const PINE = [0x5eaa57, 0x4f9a4f];
const TRUNK = 0x8b6444;

// collects instances of one shape and builds the InstancedMesh at the end
class Batch {
  constructor(geometry) { this.geometry = geometry; this.items = []; }

  add(x, y, z, sx, sy, sz, color, ry = 0) { this.items.push({ x, y, z, sx, sy, sz, color, ry }); }

  build() {
    if (!this.items.length) return null;
    const inst = track(new THREE.InstancedMesh(this.geometry, flat(0xffffff), this.items.length));
    const m4 = new THREE.Matrix4();
    const q = new THREE.Quaternion();
    const up = new THREE.Vector3(0, 1, 0);
    const c = new THREE.Color();
    this.items.forEach((it, i) => {
      q.setFromAxisAngle(up, it.ry);
      m4.compose(new THREE.Vector3(it.x, it.y, it.z), q, new THREE.Vector3(it.sx, it.sy, it.sz));
      inst.setMatrixAt(i, m4);
      inst.setColorAt(i, c.set(it.color));
    });
    inst.castShadow = true;
    inst.receiveShadow = true;
    return inst;
  }
}

// kind 0 round, 1 pine, 2 tall round. the scene is miniature, trunks stay short
function addTree(b, t) {
  const r = rng(t.seed * 13 + 5);
  const th = t.kind === 1 ? 0.5 : 0.7;
  const y0 = -DROP + 0.01;
  const rot = r() * 6;
  const c = Math.cos(rot);
  const s = Math.sin(rot);
  const put = (batch, lx, ly, lz, sx, sy, sz, color) => batch.add(t.x + lx * c + lz * s, y0 + ly, t.z - lx * s + lz * c, sx, sy, sz, color, rot);
  put(b.trunk, 0, (th * t.s) / 2, 0, t.s, th * t.s, t.s, TRUNK);
  const col = LEAF[Math.floor(r() * LEAF.length)];
  if (t.kind === 1) {
    for (let i = 0; i < 3; i++) put(b.cone, 0, (th + 0.3 + i * 0.42) * t.s, 0, (0.62 - i * 0.16) * t.s, 0.7 * t.s, (0.62 - i * 0.16) * t.s, PINE[i % 2]);
    return;
  }
  const n = t.kind === 2 ? 4 : 3;
  for (let i = 0; i < n; i++) {
    const rad = (0.52 - i * 0.06) * t.s;
    put(b.ball, (r() - 0.5) * 0.5 * t.s, (th + 0.45 + i * 0.32) * t.s, (r() - 0.5) * 0.5 * t.s, rad, rad * 0.9, rad, i === 0 ? col : LEAF[(Math.floor(r() * 9) + i) % 5]);
  }
}

export function buildPlants(plan) {
  const group = new THREE.Group();
  const b = {
    trunk: new Batch(geo('tree-trunk', () => new THREE.CylinderGeometry(0.07, 0.11, 1, 6))),
    ball: new Batch(geo('tree-ball', () => new THREE.IcosahedronGeometry(1, 1))),
    cone: new Batch(geo('tree-cone', () => new THREE.ConeGeometry(1, 1, 7))),
    tuft: new Batch(geo('lawn-tuft', () => new THREE.ConeGeometry(0.028, 0.1, 4))),
  };
  for (const t of plan.trees) addTree(b, t);
  for (const bu of plan.bushes) {
    const r = rng(bu.seed * 7 + 1);
    for (let i = 0; i < 3; i++) {
      const rad = (0.2 + r() * 0.08) * bu.s;
      b.ball.add(bu.x + (i - 1) * 0.2 * bu.s * Math.cos(bu.rot), -DROP + 0.12 * bu.s, bu.z - (i - 1) * 0.2 * bu.s * Math.sin(bu.rot), rad, rad * 0.8, rad, LEAF[Math.floor(r() * 5)]);
    }
  }
  const blade = [0x5aa24f, 0x6fb45a, 0x4f9a4f];
  plan.tufts.forEach((u, i) => {
    for (let k = 0; k < 3; k++) b.tuft.add(u.x + Math.cos(u.rot + k * 2.1) * 0.05, -DROP + 0.06 * u.s, u.z + Math.sin(u.rot + k * 2.1) * 0.05, u.s, u.s, u.s, blade[(i + k) % 3]);
  });
  for (const k of Object.keys(b)) {
    const m = b[k].build();
    if (!m) continue;
    // tufts are tiny, they do not need to cast shadows
    if (k === 'tuft') m.castShadow = false;
    group.add(m);
  }
  return group;
}
