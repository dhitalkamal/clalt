// picket fence along the lawn outlines, instanced. gaps are left where paths, bridges, water or pavilions meet it.
import * as THREE from 'three';
import { fenceParts } from './fencepts.js';
import { std, boxGeo, track } from './res.js';
import { DROP } from './terrain.js';

export function buildFence(layout, site) {
  const g = new THREE.Group();
  const { pickets, rails } = fenceParts(site);
  const total = pickets.length + rails.length * 2;
  if (!total) return g;
  const mat = std(0xfaf2e0);
  const inst = track(new THREE.InstancedMesh(boxGeo(0.05, 0.26, 0.02), mat, Math.max(1, pickets.length)));
  const m4 = new THREE.Matrix4();
  const q = new THREE.Quaternion();
  const up = new THREE.Vector3(0, 1, 0);
  const one = new THREE.Vector3(1, 1, 1);
  pickets.forEach((p, i) => {
    q.setFromAxisAngle(up, p.ry);
    m4.compose(new THREE.Vector3(p.x, -DROP + 0.13, p.z), q, one);
    inst.setMatrixAt(i, m4);
  });
  inst.count = pickets.length;
  inst.castShadow = true;
  g.add(inst);
  // rails are one unit box scaled to each run, two rows high
  const long = rails.filter((r) => Math.hypot(r.x1 - r.x0, r.z1 - r.z0) >= 0.1);
  const rail = track(new THREE.InstancedMesh(boxGeo(0.03, 0.03, 1), std(0xf3ead8), Math.max(1, long.length * 2)));
  long.forEach((r, i) => {
    const len = Math.hypot(r.x1 - r.x0, r.z1 - r.z0);
    q.setFromAxisAngle(up, Math.atan2(r.x1 - r.x0, r.z1 - r.z0));
    [0.09, 0.2].forEach((y, k) => {
      m4.compose(new THREE.Vector3((r.x0 + r.x1) / 2, -DROP + y, (r.z0 + r.z1) / 2), q, new THREE.Vector3(1, 1, len));
      rail.setMatrixAt(i * 2 + k, m4);
    });
  });
  rail.count = long.length * 2;
  g.add(rail);
  return g;
}
