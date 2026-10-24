// everything the garden creates on the gpu side goes through here, so a rebuild can free all of it at once and
// the same material or geometry is made only once. shared models from the loaded glb files are not tracked: they
// belong to the page (their meshes carry userData.shared).
import * as THREE from 'three';

let reg = { all: new Set(), cache: new Map() };

// remember anything with a dispose method and hand it back
export function track(x) {
  reg.all.add(x);
  return x;
}

// one object per key for the life of this build
export function cached(key, make) {
  if (!reg.cache.has(key)) reg.cache.set(key, track(make()));
  return reg.cache.get(key);
}

export function disposeRes() {
  for (const x of reg.all) if (x.dispose) x.dispose();
  reg = { all: new Set(), cache: new Map() };
}

export function resCount() { return reg.all.size; }

const keyOf = (extra) => Object.keys(extra).sort().map((k) => `${k}=${extra[k] && extra[k].uuid ? extra[k].uuid : extra[k]}`).join(',');

// the standard material of the garden, one per colour and option set
export const std = (color, extra = {}) => cached(`s:${color}:${keyOf(extra)}`, () => new THREE.MeshStandardMaterial({ color, roughness: 0.85, ...extra }));
export const flat = (color, extra = {}) => cached(`f:${color}:${keyOf(extra)}`, () => new THREE.MeshStandardMaterial({ color, roughness: 0.9, flatShading: true, ...extra }));
export const basic = (color, extra = {}) => cached(`b:${color}:${keyOf(extra)}`, () => new THREE.MeshBasicMaterial({ color, toneMapped: false, ...extra }));

// shared geometries by shape and size
export const boxGeo = (w, h, d) => cached(`g:box:${w}:${h}:${d}`, () => new THREE.BoxGeometry(w, h, d));
export const cylGeo = (rt, rb, h, seg = 12) => cached(`g:cyl:${rt}:${rb}:${h}:${seg}`, () => new THREE.CylinderGeometry(rt, rb, h, seg));
export const geo = (key, make) => cached('g:' + key, make);

export const shade = (o) => { o.traverse((m) => { if (m.isMesh) { m.castShadow = true; m.receiveShadow = true; } }); return o; };

export const box = (w, h, d, color, extra) => {
  const m = new THREE.Mesh(boxGeo(w, h, d), cached(`s:${color}:${keyOf(extra || {})}:78`, () => new THREE.MeshStandardMaterial({ color, roughness: 0.78, ...extra })));
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
};

export const cyl = (rt, rb, h, color, seg = 12, extra) => {
  const m = new THREE.Mesh(cylGeo(rt, rb, h, seg), cached(`s:${color}:${keyOf(extra || {})}:78`, () => new THREE.MeshStandardMaterial({ color, roughness: 0.78, ...extra })));
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
};

export const at = (o, x, y, z, ry = 0) => { o.position.set(x, y, z); o.rotation.y = ry; return o; };
