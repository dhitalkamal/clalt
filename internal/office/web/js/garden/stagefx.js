// the garden wants its own sky, light and shadow size. this saves what the page stage had and puts it back when the
// hall is built again, so the hall renders exactly as before.
import * as THREE from 'three';
import { SKY } from './terrain.js';
import { fogRange } from './cameras.js';
import { setStaticShadows } from '../agentShadow.js';

export const SHADOW_SIZE = 2048;
let saved = null;

const hemiOf = (scene) => scene.children.find((o) => o.isHemisphereLight);

function setShadowSize(sun, size) {
  if (sun.shadow.mapSize.x === size) return;
  sun.shadow.mapSize.set(size, size);
  // the old map has the old size, free it so three makes a new one
  if (sun.shadow.map) { sun.shadow.map.dispose(); sun.shadow.map = null; }
}

export function enterGardenStage(stage) {
  if (saved) return;
  const { scene, renderer, sun, camera } = stage;
  const hemi = hemiOf(scene);
  saved = {
    clear: renderer.getClearColor(new THREE.Color()).getHex(), fog: scene.fog.color.getHex(), near: scene.fog.near, far: scene.fog.far,
    hemi: hemi ? [hemi.color.getHex(), hemi.groundColor.getHex(), hemi.intensity] : null, env: scene.environmentIntensity,
    exposure: renderer.toneMappingExposure, sun: [sun.color.getHex(), sun.intensity, sun.shadow.radius], size: sun.shadow.mapSize.x,
    fov: camera.fov, far2: camera.far,
  };
  renderer.setClearColor(SKY, 1);
  scene.fog.color.setHex(SKY);
  if (hemi) { hemi.color.setHex(0xdcefff); hemi.groundColor.setHex(0x86b565); hemi.intensity = 1.0; }
  scene.environmentIntensity = 0.5;
  renderer.toneMappingExposure = 1.0;
  sun.color.setHex(0xfff4de);
  sun.intensity = 3.0;
  sun.shadow.radius = 6;
  setShadowSize(sun, SHADOW_SIZE);
  // static geometry only: agents get a floor disc instead of a live shadow
  renderer.shadowMap.autoUpdate = false;
  renderer.shadowMap.needsUpdate = true;
  setStaticShadows(true);
}

export function leaveGardenStage(stage) {
  if (!saved) return;
  const { scene, renderer, sun, camera } = stage;
  const s = saved;
  saved = null;
  renderer.setClearColor(s.clear, 1);
  scene.fog.color.setHex(s.fog);
  scene.fog.near = s.near;
  scene.fog.far = s.far;
  const hemi = hemiOf(scene);
  if (hemi && s.hemi) { hemi.color.setHex(s.hemi[0]); hemi.groundColor.setHex(s.hemi[1]); hemi.intensity = s.hemi[2]; }
  scene.environmentIntensity = s.env;
  renderer.toneMappingExposure = s.exposure;
  sun.color.setHex(s.sun[0]);
  sun.intensity = s.sun[1];
  sun.shadow.radius = s.sun[2];
  setShadowSize(sun, s.size);
  renderer.shadowMap.autoUpdate = true;
  setStaticShadows(false);
  camera.fov = s.fov;
  camera.far = s.far2;
  camera.updateProjectionMatrix();
}

// fits the sun and its shadow box to the island, the fog to the camera distance
export function fitLight(stage, bb, dist) {
  const cx = (bb.x0 + bb.x1) / 2;
  const cz = (bb.z0 + bb.z1) / 2;
  const R = Math.max(bb.x1 - bb.x0, bb.z1 - bb.z0) * 0.62;
  const { sun } = stage;
  sun.position.set(cx - R * 0.8, R * 1.2, cz - R * 0.2);
  sun.target.position.set(cx, 0, cz);
  const sc = sun.shadow.camera;
  sc.left = -R; sc.right = R; sc.top = R; sc.bottom = -R;
  sc.near = 0.1; sc.far = R * 4;
  sc.updateProjectionMatrix();
  stage.renderer.shadowMap.needsUpdate = true;
  fitFog(stage, dist);
}

// the fog and far plane that suit a camera distance
export function fitFog({ scene, camera }, dist) {
  const f = fogRange(dist);
  scene.fog.near = f.near;
  scene.fog.far = f.far;
  camera.far = f.camFar;
  camera.updateProjectionMatrix();
}
