// one ray per frame from the head to the camera against the layout's static meshes. needs three.js, ground.js
// loads it lazily. the mesh list is cached per layout: big opaque meshes only (walls, trees, pavilions, decks),
// no furniture, agents, avatar, instanced scatter, sprites or glass.
import * as THREE from 'three';

const MIN_SIZE = 0.8; // smaller meshes (props, chairs) never block the camera
const MIN_TOP = 1.0; // and a mesh has to reach head height

function skipRoots(ctx) {
  const roots = new Set();
  for (const a of ctx.app?.agents ? ctx.app.agents.values() : []) for (const k of ['rig', 'model', 'flair']) if (a[k]) roots.add(a[k]);
  return roots;
}

// meshes that can block the camera. exported for the page check
export function collectStatics(ctx) {
  const roots = skipRoots(ctx);
  const out = [];
  const box = new THREE.Box3();
  const size = new THREE.Vector3();
  ctx.stage.scene.updateMatrixWorld(true);
  const walk = (o) => {
    // furniture holders (userData.itemId) never block the camera, only the shell, trees, pavilions and decks
    if (!o.visible || roots.has(o) || o.userData?.noCamHit || o.userData?.itemId) return;
    if (o.isMesh && !o.isInstancedMesh && !o.isSkinnedMesh) {
      const m = [].concat(o.material)[0];
      if (!(m && m.transparent && m.opacity < 0.6) && !(m && m.visible === false)) {
        box.setFromObject(o);
        box.getSize(size);
        if (Math.max(size.x, size.y, size.z) >= MIN_SIZE && box.max.y >= MIN_TOP) out.push(o);
      }
    }
    for (const c of o.children) walk(c);
  };
  walk(ctx.stage.scene);
  return out;
}

export function createCamRay(ctx) {
  const ray = new THREE.Raycaster();
  const from = new THREE.Vector3();
  const dir = new THREE.Vector3();
  let list = null;
  let key = null;
  const keyOf = () => ({ layout: ctx.app?.layout, n: ctx.stage.scene.children.length });
  return {
    // distance from a to b to the first static mesh, or null. a and b are [x, y, z]
    hit(a, b) {
      const k = keyOf();
      if (!list || !key || key.layout !== k.layout || key.n !== k.n) { list = collectStatics(ctx); key = k; }
      from.set(a[0], a[1], a[2]);
      dir.set(b[0] - a[0], b[1] - a[1], b[2] - a[2]);
      const len = dir.length();
      if (len < 1e-6) return null;
      dir.divideScalar(len);
      ray.set(from, dir);
      ray.near = 0;
      ray.far = len;
      const h = ray.intersectObjects(list, false)[0];
      return h ? h.distance : null;
    },
    // distance straight up from p to a static mesh (a deck overhead), or null within max
    above(p, max) {
      if (!list) return null;
      ray.set(from.set(p[0], p[1], p[2]), dir.set(0, 1, 0));
      ray.near = 0;
      ray.far = max;
      const h = ray.intersectObjects(list, false)[0];
      return h ? h.distance : null;
    },
  };
}
