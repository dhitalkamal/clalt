// a small flat ring on the floor under an agent, in the colour of its account
import * as THREE from 'three';
import { app } from './app.js';
import { ringColor } from './model.js';

const OPACITY = 0.9;
const p = new THREE.Vector3();
let ringGeo = null;

// the ring of one agent, or null when it has no account. the geometry is shared, the material is its own
export function makeAccountRing(a, world) {
  const color = ringColor(a.s);
  if (color == null) return null;
  const geo = ringGeo || (ringGeo = new THREE.RingGeometry(0.26, 0.38, 32));
  const mat = new THREE.MeshBasicMaterial({ color, transparent: true, opacity: OPACITY, depthWrite: false, toneMapped: false, side: THREE.DoubleSide });
  const m = new THREE.Mesh(geo, mat);
  m.rotation.x = -Math.PI / 2;
  m.renderOrder = 2;
  m.frustumCulled = false;
  // the geometry belongs to the page: disposeTree frees only the material
  m.userData.shared = true;
  m.userData.noCamHit = true;
  world.add(m);
  return m;
}
// under the feet, like the shadow disc. fades with the agent while the build editor is open
export function placeAccountRing(a) {
  const m = a.groundRing;
  if (!m) return;
  a.model.getWorldPosition(p);
  const y = (a.mode === 'seated' ? a.rig.position.y : p.y) + 0.03;
  m.position.set(p.x, y, p.z);
  m.scale.setScalar(Math.max(a.scale, 0.001));
  m.visible = a.mode !== 'away' || a.scale > 0.05;
  m.material.opacity = app.editing ? OPACITY * 0.45 : OPACITY;
}

export function disposeAccountRing(a) {
  const m = a.groundRing;
  if (!m) return;
  a.world.remove(m);
  m.material.dispose();
}
