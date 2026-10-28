// folds the small static meshes of a group into one mesh per material, so a pavilion or a stretch of scenery costs a
// few draw calls instead of hundreds. looks the same: same materials, same shadow flags, same transforms baked in.
// what stays alone: instanced meshes, skinned ones, material arrays, transparent ones, anything with userData (the
// animated and picked parts carry it) and the big meshes the camera collision looks at (see mergekey.js)
import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { mergeable, groupMergeable } from './mergekey.js';
import { track } from './res.js';

const box = new THREE.Box3();
const size = new THREE.Vector3();
const inv = new THREE.Matrix4();

// plain view of a mesh for the pure grouping code
const shaped = (m) => {
  const g = m.geometry;
  return { isMesh: m.isMesh, isInstancedMesh: m.isInstancedMesh, isSkinnedMesh: m.isSkinnedMesh, visible: m.visible, material: m.material,
    castShadow: m.castShadow, receiveShadow: m.receiveShadow, renderOrder: m.renderOrder, userData: m.userData, src: m,
    geometry: { attrs: Object.keys(g.attributes), indexed: !!g.index } };
};

function collect(group) {
  const out = [];
  const walk = (o) => {
    // a branch with userData is animated or looked up by id: leave all of it alone
    if (o !== group && o.userData && Object.keys(o.userData).length) return;
    if (o.isMesh) {
      box.setFromObject(o);
      box.getSize(size);
      const flipped = o.matrixWorld.determinant() < 0;
      const s = shaped(o);
      if (!flipped && o.geometry && mergeable(s, { size: Math.max(size.x, size.y, size.z), top: box.max.y })) out.push(s);
    }
    for (const c of o.children) walk(c);
  };
  walk(group);
  return out;
}

// merges what it can, returns how many meshes went away
export function mergeStatic(group) {
  group.updateWorldMatrix(true, true);
  inv.copy(group.matrixWorld).invert();
  let removed = 0;
  for (const { meshes } of groupMergeable(collect(group))) {
    const parts = meshes.map((s) => s.src);
    const geos = parts.map((m) => m.geometry.clone().applyMatrix4(new THREE.Matrix4().multiplyMatrices(inv, m.matrixWorld)));
    const merged = mergeGeometries(geos, false);
    geos.forEach((g) => g.dispose());
    if (!merged) continue;
    const first = parts[0];
    const out = new THREE.Mesh(track(merged), first.material);
    out.castShadow = first.castShadow;
    out.receiveShadow = first.receiveShadow;
    out.renderOrder = first.renderOrder;
    // small parts are not camera blockers one by one, so the folded mesh must not become one
    out.userData.noCamHit = true;
    out.userData.merged = parts.length;
    group.add(out);
    for (const m of parts) m.parent?.remove(m);
    removed += parts.length - 1;
  }
  return removed;
}
