// the garden bakes its sun shadows once, so a walking agent cannot cast one. each agent gets a soft dark disc on the
// floor instead: one shared geometry, material and texture for all of them.
import * as THREE from 'three';

let on = false;
let shared = null;

export const staticShadows = () => on;
export const setStaticShadows = (v) => { on = !!v; };

function blobParts() {
  if (shared) return shared;
  const c = document.createElement('canvas');
  c.width = c.height = 64;
  const g = c.getContext('2d');
  const grad = g.createRadialGradient(32, 32, 2, 32, 32, 31);
  grad.addColorStop(0, 'rgba(0,0,0,0.55)');
  grad.addColorStop(0.6, 'rgba(0,0,0,0.3)');
  grad.addColorStop(1, 'rgba(0,0,0,0)');
  g.fillStyle = grad;
  g.fillRect(0, 0, 64, 64);
  const map = new THREE.CanvasTexture(c);
  const mat = new THREE.MeshBasicMaterial({ map, transparent: true, depthWrite: false, toneMapped: false, polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2 });
  const geo = new THREE.PlaneGeometry(1, 1).rotateX(-Math.PI / 2);
  shared = { mat, geo };
  return shared;
}

// the disc of one agent, or null when the stage still draws live shadows (the hall)
export function makeBlob(world) {
  if (!on) return null;
  const { mat, geo } = blobParts();
  const m = new THREE.Mesh(geo, mat);
  m.renderOrder = 1;
  m.frustumCulled = false;
  // shared parts belong to the page: disposeTree leaves them alone
  m.userData.shared = true;
  m.userData.noCamHit = true;
  world.add(m);
  return m;
}

// agents do not cast live shadows in a garden
export function stripCast(a) {
  for (const part of [a.rig, a.model]) part.traverse((o) => { if (o.isMesh) o.castShadow = false; });
}

const p = new THREE.Vector3();

// under the feet: seated agents sit on the chair, so the floor is the seat slot's height
export function placeBlob(a) {
  const m = a.blob;
  if (!m) return;
  a.model.getWorldPosition(p);
  const y = (a.mode === 'seated' ? a.rig.position.y : p.y) + 0.025;
  m.position.set(p.x, y, p.z);
  const s = 0.62 * Math.max(a.scale, 0.001);
  m.scale.set(s, 1, s);
  m.visible = a.mode !== 'away' || a.scale > 0.05;
}
