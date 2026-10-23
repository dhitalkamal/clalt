import * as THREE from 'three';
import { SKY } from './ground.js';
import { RoomEnvironment } from 'three/addons/environments/RoomEnvironment.js';

// renderer, lights, camera and controls. no scene content in here.
export function createStage(canvas) {
  const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, powerPreference: 'high-performance' });
  // cap at 2: crisp on 4K and retina, without paying for dpr 3 on phones
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.05;
  renderer.shadowMap.enabled = true;
  renderer.shadowMap.type = THREE.PCFSoftShadowMap;

  const scene = new THREE.Scene();
  renderer.setClearColor(SKY, 1);
  scene.fog = new THREE.Fog(SKY, 38, 105);
  const pmrem = new THREE.PMREMGenerator(renderer);
  scene.environment = pmrem.fromScene(new RoomEnvironment(), 0.04).texture;
  scene.environmentIntensity = 0.45;

  scene.add(new THREE.HemisphereLight(0xfff1de, 0x6a5a4c, 0.9));

  // warm sun coming through the left and back windows
  const sun = new THREE.DirectionalLight(0xffcf94, 3.2);
  sun.castShadow = true;
  sun.shadow.mapSize.set(4096, 4096);
  sun.shadow.bias = -0.0003;
  sun.shadow.normalBias = 0.012;
  sun.shadow.radius = 4;
  scene.add(sun, sun.target);

  const camera = new THREE.PerspectiveCamera(30, 1, 0.1, 400);
  // one fixed camera: no orbit, no pan. the controls object only holds the look-at target for the wheel zoom.
  const controls = { target: new THREE.Vector3(), update() { camera.lookAt(this.target); } };

  return { renderer, scene, camera, controls, sun, canvas, zoom: { min: 0.4, base: 30 } };
}

// view presets and the orbit state. start view: 75 degrees down, front view: 32 degrees.
export const ELEV = THREE.MathUtils.degToRad(75);
export const PRESETS = { top: THREE.MathUtils.degToRad(75), front: THREE.MathUtils.degToRad(32) };
export const EL_MIN = THREE.MathUtils.degToRad(20);
export const EL_MAX = THREE.MathUtils.degToRad(85);
export const VIEW_DIR = new THREE.Vector3(0, Math.sin(ELEV), Math.cos(ELEV));

const planeY = new THREE.Plane(new THREE.Vector3(0, 1, 0), -0.3);
const hitA = new THREE.Vector3();
const hitB = new THREE.Vector3();
const ray = new THREE.Raycaster();

function floorPoint(stage, ndc, out) {
  ray.setFromCamera(ndc, stage.camera);
  return ray.ray.intersectPlane(planeY, out);
}

// fits the sun shadow box and the camera to the whole office for each preset, then starts in the top preset
export function frameRoom(stage, W, D, aspect) {
  const { camera, sun } = stage;
  const R = Math.max(W, D) * 0.8;
  sun.position.set(-R * 0.8, R * 1.1, -R * 0.2);
  sun.target.position.set(0, 0, 0);
  const sc = sun.shadow.camera;
  sc.left = -R; sc.right = R; sc.top = R; sc.bottom = -R;
  sc.near = 0.1; sc.far = R * 4;
  sc.updateProjectionMatrix();

  const tv = Math.tan(THREE.MathUtils.degToRad(camera.fov) / 2);
  const th = tv * aspect;
  const fit = (el) => {
    const needH = W / 2 + 1.2;
    const needV = (D * Math.sin(el) + 2.6 * Math.cos(el)) / 2 + 0.9;
    return Math.max(needH / th, needV / tv) * 1.04;
  };
  stage.fitDist = { top: fit(PRESETS.top), front: fit(PRESETS.front) };
  stage.base = { dist: stage.fitDist.top, target: new THREE.Vector3(0, 0.6, 0) };
  stage.bounds = { x: W / 2, z: D / 2 };
  stage.vel = { az: 0, el: 0 };
  setPreset(stage, 'top');
  stage.scene.fog.near = stage.fitDist.top * 1.8;
  stage.scene.fog.far = stage.fitDist.top * 5.5;
}

export function setPreset(stage, name) {
  stage.preset = name;
  stage.view = { dist: stage.fitDist[name], target: stage.base.target.clone(), az: 0, el: PRESETS[name] };
  stage.vel = { az: 0, el: 0 };
  applyView(stage);
}

export function applyView(stage) {
  const { camera, controls, view } = stage;
  const ce = Math.cos(view.el);
  VIEW_DIR.set(Math.sin(view.az) * ce, Math.sin(view.el), Math.cos(view.az) * ce);
  controls.target.copy(view.target);
  camera.position.copy(view.target).addScaledVector(VIEW_DIR, view.dist);
  camera.lookAt(view.target);
  camera.updateMatrixWorld();
}

const clampTarget = (stage) => {
  const b = stage.bounds;
  const t = stage.view.target;
  t.x = Math.max(-b.x, Math.min(b.x, t.x));
  t.z = Math.max(-b.z, Math.min(b.z, t.z));
};

// drag orbit: full 360 azimuth, pitch clamped so the camera never goes under the floor or flips
export function orbit(stage, dx, dy) {
  const v = stage.view;
  const daz = -dx * 0.006;
  const del = dy * 0.005;
  v.az += daz;
  v.el = Math.max(EL_MIN, Math.min(EL_MAX, v.el + del));
  stage.vel = { az: daz, el: del };
  applyView(stage);
}

// inertia after release
export function tickCamera(stage, dragging) {
  if (dragging) return;
  const w = stage.vel;
  if (Math.abs(w.az) + Math.abs(w.el) < 1e-4) return;
  const v = stage.view;
  v.az += w.az;
  v.el = Math.max(EL_MIN, Math.min(EL_MAX, v.el + w.el));
  w.az *= 0.9; w.el *= 0.9;
  applyView(stage);
}

// pan along the ground, relative to the current heading
export function pan(stage, dx, dy) {
  const v = stage.view;
  const s = v.dist * 0.0016;
  v.target.x += (-dx * Math.cos(v.az) - dy * Math.sin(v.az)) * s;
  v.target.z += (dx * Math.sin(v.az) - dy * Math.cos(v.az)) * s;
  clampTarget(stage);
  applyView(stage);
}

// wheel and pinch zoom toward the cursor, clamped
export function zoomAt(stage, ndc, factor) {
  const v = stage.view;
  const before = floorPoint(stage, ndc, hitA) ? hitA.clone() : null;
  v.dist = Math.min(stage.base.dist * 1.3, Math.max(stage.base.dist * 0.25, v.dist * factor));
  applyView(stage);
  if (before && floorPoint(stage, ndc, hitB)) {
    v.target.add(before.sub(hitB));
    clampTarget(stage);
    applyView(stage);
  }
}

export function resetZoom(stage) { setPreset(stage, 'top'); }

// jump the zoom to a world point, keeping the current heading (stills use this)
export function zoomTo(stage, x, y, z, frac) {
  stage.view.target.set(x, y, z);
  stage.view.dist = stage.base.dist * frac;
  applyView(stage);
}

export function setView(stage, pos, target) {
  stage.camera.position.copy(pos);
  stage.controls.target.copy(target);
  stage.camera.lookAt(target);
  stage.camera.updateMatrixWorld();
}
