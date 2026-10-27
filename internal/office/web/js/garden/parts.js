// shared building blocks: decks, plazas, posts, awnings, pergolas, picket runs
import * as THREE from 'three';
import { std, box, cached, boxGeo, track, geo, at } from './res.js';
import { plankTex, flagTex, stripeTex } from './tex.js';
import { stringLights } from './propmesh.js';

export const WOOD = 0x9a6a3e;
export const POST = 0x8b5e3c;
const shadeCol = (hex, k) => new THREE.Color(hex).multiplyScalar(k).getHex();

// slab with a textured top and plain trim sides. top face at y = 0, thickness t going down.
export function slab(w, d, map, t, side) {
  const top = cached(`m:slab:${map.uuid}`, () => new THREE.MeshStandardMaterial({ map, roughness: 0.9 }));
  const sd = std(side);
  const m = new THREE.Mesh(geo(`slab:${w}:${t}:${d}`, () => new THREE.BoxGeometry(w, t, d).translate(0, -t / 2, 0)), [sd, sd, top, sd, sd, sd]);
  m.receiveShadow = true;
  m.castShadow = true;
  return m;
}

// wood deck, top face at y = 0, trim in a darker tone of the zone colour
export function deck(w, d, hue, light, trim, t = 0.1) {
  return slab(w, d, plankTex(w, d, hue, light), t, shadeCol(trim, 0.7));
}

export function plaza(w, d, t = 0.1) {
  return slab(w, d, flagTex(w, d), t, 0x9b9488);
}

export const post = (h, w = 0.07, color = POST) => {
  const m = box(w, h, w, color);
  m.position.y = h / 2;
  return m;
};

// striped awning, tilted a little toward the viewer
export function awning(w, d, tint, tilt = 0.22) {
  const t = stripeTex(tint, 8, Math.max(1, Math.round(w / 0.5) / 8));
  const m = new THREE.Mesh(boxGeo(w, 0.025, d), std(0xffffff, { map: t, roughness: 0.9 }));
  m.rotation.x = tilt;
  m.castShadow = true;
  m.receiveShadow = true;
  const frill = new THREE.Mesh(boxGeo(w, 0.05, 0.015), std(tint));
  frill.position.set(0, -0.015, (d / 2) * Math.cos(tilt));
  m.add(frill);
  return m;
}

// open roof frame: corner posts, two long beams, slats, string lights. no solid roof so agents stay visible.
export function pergola(x0, z0, x1, z1, H, o = {}) {
  const g = new THREE.Group();
  const wood = o.wood || WOOD;
  const xs = [x0, x1, ...(o.postsX || [])];
  for (const x of xs) for (const z of [z0, z1]) { const p = post(H, 0.08, wood); p.position.x = x; p.position.z = z; g.add(p); }
  const beam = (w, d, x, z) => { const m = box(w, 0.06, d, wood); m.position.set(x, H, z); g.add(m); };
  beam(x1 - x0 + 0.2, 0.08, (x0 + x1) / 2, z0);
  beam(x1 - x0 + 0.2, 0.08, (x0 + x1) / 2, z1);
  const step = o.step || 0.55;
  const slats = Math.max(1, Math.ceil((x1 - x0 - step / 2) / step));
  const slat = track(new THREE.InstancedMesh(boxGeo(0.04, 0.04, z1 - z0 + 0.2), std(wood), slats));
  const m4 = new THREE.Matrix4();
  for (let i = 0; i < slats; i++) { m4.setPosition(x0 + step / 2 + i * step, H + 0.05, (z0 + z1) / 2); slat.setMatrixAt(i, m4); }
  slat.castShadow = true;
  g.add(slat);
  g.add(stringLights({ x: x0, y: H - 0.04, z: z1 }, { x: x1, y: H - 0.04, z: z1 }, 0.14));
  g.add(stringLights({ x: x0, y: H - 0.04, z: z1 }, { x: x0, y: H - 0.04, z: z0 }, 0.12));
  g.add(stringLights({ x: x1, y: H - 0.04, z: z1 }, { x: x1, y: H - 0.04, z: z0 }, 0.12));
  return g;
}

// low white picket run, optional gap along the run. pickets are one instanced mesh per run
export function pickets(ax, az, bx, bz, gapA = 0, gapB = 0) {
  const g = new THREE.Group();
  const len = Math.hypot(bx - ax, bz - az);
  const n = Math.floor(len / 0.12);
  const horiz = Math.abs(bz - az) < 1e-6;
  const mat = std(0xf6efe0);
  const keep = [];
  for (let i = 0; i <= n; i++) {
    const t = i / n;
    const x = ax + (bx - ax) * t;
    const z = az + (bz - az) * t;
    const u = horiz ? x : z;
    if (gapB > gapA && u > gapA && u < gapB) continue;
    keep.push([x, z]);
  }
  const inst = track(new THREE.InstancedMesh(boxGeo(horiz ? 0.045 : 0.02, 0.26, horiz ? 0.02 : 0.045), mat, keep.length));
  const m4 = new THREE.Matrix4();
  keep.forEach(([x, z], i) => { m4.setPosition(x, 0.14, z); inst.setMatrixAt(i, m4); });
  inst.castShadow = true;
  g.add(inst);
  const rail = new THREE.Mesh(boxGeo(horiz ? len : 0.03, 0.025, horiz ? 0.03 : len), mat);
  rail.position.set((ax + bx) / 2, 0.2, (az + bz) / 2);
  g.add(rail);
  return g;
}

export { at };
