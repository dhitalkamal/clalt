// fades the hall walls the camera looks through. every piece tagged userData.cut by shell.js gets its own copy of
// its materials, and update() eases their opacity from the camera each frame (math in nav/cutmath.js).
import * as THREE from 'three';
import { wallOpacity, stepFade } from '../nav/cutmath.js';

const LOW = 0.1;
const dir = new THREE.Vector3();

export function createCutaway(root) {
  const parts = [];
  root.traverse((o) => {
    if (!o.userData.cut) return;
    const meshes = [];
    o.traverse((m) => {
      if (!m.isMesh) return;
      m.material = Array.isArray(m.material) ? m.material.map((x) => x.clone()) : m.material.clone();
      meshes.push({ m, mats: [].concat(m.material).map((x) => ({ x, op: x.opacity, tr: x.transparent, dw: x.depthWrite })) });
    });
    parts.push({ wall: o.userData.cut, meshes, k: 1 });
  });

  // returns true while any wall is still fading, so callers can skip work when everything is settled
  function update(camera, dt, reduced = false) {
    camera.getWorldDirection(dir);
    const cam = { x: camera.position.x, z: camera.position.z, el: Math.asin(Math.max(-1, Math.min(1, -dir.y))) };
    let busy = false;
    for (const part of parts) {
      const want = wallOpacity({ cam, wall: part.wall, low: LOW });
      const k = stepFade(part.k, want, dt, reduced);
      if (k === part.k) continue;
      part.k = k;
      busy = true;
      for (const { mats } of part.meshes) {
        for (const e of mats) {
          e.x.opacity = e.op * k;
          const fading = k < 0.999;
          if (e.x.transparent !== (fading || e.tr)) { e.x.transparent = fading || e.tr; e.x.needsUpdate = true; }
          e.x.depthWrite = fading ? false : e.dw;
        }
      }
    }
    return busy;
  }

  return { update, parts };
}
