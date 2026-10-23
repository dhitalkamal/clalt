import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { clone } from 'three/addons/utils/SkeletonUtils.js';

const FURNITURE = ['desk', 'chairDesk', 'computerScreen', 'computerKeyboard', 'computerMouse', 'laptop',
  'lampRoundFloor', 'lampSquareTable', 'lampSquareFloor', 'pottedPlant', 'plantSmall1', 'plantSmall2',
  'bookcaseClosed', 'bookcaseOpen', 'rugRectangle', 'floorFull', 'wall', 'wallWindow', 'wallDoorway',
  'wallCorner', 'doorwayFront', 'tableCoffee', 'loungeSofa', 'books', 'cabinetTelevision',
  'kitchenCoffeeMachine', 'sideTable', 'tableRound', 'rugDoormat', 'rugRound', 'loungeChair', 'kitchenCabinet', 'kitchenCabinetDrawer'];
export const CHARACTERS = ['female-a', 'male-a', 'female-b', 'male-b', 'female-c', 'male-c',
  'female-d', 'male-d', 'female-e', 'male-e', 'female-f', 'male-f'];

const cache = new Map();
const loader = new GLTFLoader();

function load(path) {
  return new Promise((resolve, reject) => loader.load(path, resolve, undefined, reject));
}

export async function loadAll(onProgress) {
  const jobs = [
    ...FURNITURE.map((n) => ['f:' + n, `assets/furniture/${n}.glb`]),
    ...CHARACTERS.map((n) => ['c:' + n, `assets/characters/character-${n}.glb`]),
  ];
  let done = 0;
  await Promise.all(jobs.map(async ([key, path]) => {
    cache.set(key, await load(path));
    onProgress?.(++done / jobs.length);
  }));
}

// wrapper whose origin is the bottom-center of the model, so placement is predictable
export function furniture(name, opts = {}) {
  const src = cache.get('f:' + name);
  const inner = clone(src.scene);
  const box = new THREE.Box3().setFromObject(inner);
  const c = box.getCenter(new THREE.Vector3());
  inner.position.set(-c.x, -box.min.y, -c.z);
  const g = new THREE.Group();
  g.add(inner);
  g.userData.size = box.getSize(new THREE.Vector3());
  if (opts.scale) g.scale.setScalar(opts.scale);
  g.traverse((o) => { if (o.isMesh) { o.castShadow = opts.noCast ? false : true; o.receiveShadow = true; o.userData.shared = true; } });
  return g;
}

export function character(name) {
  const src = cache.get('c:' + name);
  const model = clone(src.scene);
  model.traverse((o) => { if (o.isMesh) { o.castShadow = true; o.receiveShadow = true; o.frustumCulled = false; o.userData.shared = true; } });
  return { model, clips: src.animations };
}
