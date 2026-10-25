// scene rays for the sky camera: the point under the cursor (orbit pivot) and the wall between pivot and camera.
// cast(o, d, far, floors) -> {distance, point} | null is injected, the page builds it from three.js (threeCast).
let THREE = null;
if (typeof window !== 'undefined') import('three').then((m) => { THREE = m; }).catch(() => {});

const WALL_TOP = 14;

export function createPicker(ctx, cast = null) {
  const caster = () => cast || (THREE ? (cast = threeCast(THREE, ctx)) : null);
  return {
    // surface under a screen ray, only when it is within reach of the camera
    pivotAt(ray, reach) {
      const c = caster();
      const h = c && c(ray.o, ray.d, reach, true);
      return h ? h.point : null;
    },
    // distance along o -> d to the first wall (floors are ignored), or null
    wallAlong(o, d, far) {
      const c = caster();
      const h = c && c(o, d, far, false);
      return h ? h.distance : null;
    },
  };
}

// raycaster over the scene: no agents (skinned meshes are slow to hit and not walls), no sprites, no glass, no sky dome
export function threeCast(T, ctx) {
  const rc = new T.Raycaster();
  const o = new T.Vector3(); const d = new T.Vector3(); const n = new T.Vector3();
  return (org, dir, far, floors) => {
    const { scene, camera } = ctx.stage;
    if (!scene) return null;
    const hidden = [];
    for (const a of ctx.app && ctx.app.agents ? ctx.app.agents.values() : []) {
      if (a && a.model) a.model.traverse((x) => { if (x.layers.test(rc.layers)) { x.layers.disable(0); hidden.push(x); } });
    }
    try {
      rc.camera = camera;
      rc.near = 0; rc.far = far;
      rc.set(o.set(org.x, org.y, org.z), d.set(dir.x, dir.y, dir.z));
      for (const h of rc.intersectObject(scene, true)) {
        const m = h.object;
        const mat = Array.isArray(m.material) ? m.material[0] : m.material;
        if (!m.isMesh || !m.visible || !mat || mat.side === T.BackSide || (mat.transparent && mat.opacity < 0.5)) continue;
        // clouds and other high props are not walls
        if (!floors && h.point.y > WALL_TOP) continue;
        if (!floors && h.face && n.copy(h.face.normal).transformDirection(m.matrixWorld).y > 0.6) continue;
        return { distance: h.distance, point: { x: h.point.x, y: h.point.y, z: h.point.z } };
      }
      return null;
    } finally { for (const x of hidden) x.layers.enable(0); }
  };
}
