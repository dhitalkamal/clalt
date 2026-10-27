import * as THREE from 'three';

// small helper: put an object into a group at x,y,z with a yaw. the extra arguments are placement tags from the
// demo's alignment check and are ignored.
export function makeBuilder() {
  const g = new THREE.Group();
  const ticks = [];
  const put = (obj, x, y, z, ry = 0) => {
    obj.position.set(x, y, z);
    obj.rotation.y = ry;
    g.add(obj);
    return obj;
  };
    return { g, ticks, put, track: (obj) => obj, swayList: [] };
}
export const glass = (tint, op = 0.2) => new THREE.MeshPhysicalMaterial({ color: tint, transparent: true, opacity: op, roughness: 0.1, side: THREE.DoubleSide, depthWrite: false });
