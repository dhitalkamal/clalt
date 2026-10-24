// small garden objects: benches, lamps, flower beds, ponds, the fountain, string lights, standing tables, signs
import * as THREE from 'three';
import { rng } from './geo.js';
import { std, flat, box, cyl, shade, geo, cached, track, boxGeo, at } from './res.js';

const FLOWER = [0xf2c14b, 0xe8647c, 0xffffff, 0xa77be0, 0xf08a4b, 0x6bb6ee];
const mesh = (g, m) => new THREE.Mesh(g, m);

// raised bed with soil and flower heads, drawn as two instanced meshes. w x d footprint, origin at the centre.
export function flowerBed(w, d, seed) {
  const r = rng(seed * 31 + 9);
  const g = new THREE.Group();
  const frame = mesh(boxGeo(w, 0.1, d), std(0x9a6a3e));
  frame.position.y = 0.05;
  const soil = mesh(boxGeo(w - 0.1, 0.02, d - 0.1), std(0x5b3d28));
  soil.position.y = 0.105;
  g.add(frame, soil);
  const n = Math.max(3, Math.round(w * d * 14));
  const stems = track(new THREE.InstancedMesh(geo('stem', () => new THREE.CylinderGeometry(0.006, 0.006, 0.1, 4)), std(0x4f8f45), n));
  const heads = track(new THREE.InstancedMesh(geo('flower-head', () => new THREE.IcosahedronGeometry(0.035, 0)), flat(0xffffff), n));
  const m4 = new THREE.Matrix4();
  const col = new THREE.Color();
  for (let i = 0; i < n; i++) {
    const x = (r() - 0.5) * (w - 0.18);
    const z = (r() - 0.5) * (d - 0.18);
    const h = 0.08 + r() * 0.1;
    m4.compose(new THREE.Vector3(x, 0.1 + h / 2, z), new THREE.Quaternion(), new THREE.Vector3(1, h / 0.1, 1));
    stems.setMatrixAt(i, m4);
    m4.identity().setPosition(x, 0.1 + h, z);
    heads.setMatrixAt(i, m4);
    heads.setColorAt(i, col.set(FLOWER[(i + Math.floor(r() * 3)) % FLOWER.length]));
  }
  g.add(stems, heads);
  return shade(g);
}

export function bench(color = 0xb98a5e) {
  const g = new THREE.Group();
  const seat = box(0.8, 0.05, 0.26, color);
  seat.position.y = 0.26;
  const back = box(0.8, 0.2, 0.03, color);
  back.position.set(0, 0.42, -0.12);
  g.add(seat, back);
  for (const dx of [-0.34, 0.34]) { const l = box(0.05, 0.26, 0.22, 0x6b5a48); l.position.set(dx, 0.13, 0); g.add(l); }
  return g;
}

export function rock(s, seed) {
  const m = mesh(geo('rock', () => new THREE.DodecahedronGeometry(0.12, 0)), flat(0x9aa0a0));
  m.scale.set(s, s * 0.6, s);
  m.rotation.set(seed, seed * 2, 0);
  m.position.y = 0.05 * s;
  return shade(m);
}

// wooden lamp post with a glass lantern, height 1.25
export function lampPost() {
  const g = new THREE.Group();
  const post = cyl(0.03, 0.04, 1.25, 0x5c4330, 6);
  post.position.y = 0.625;
  const lantern = mesh(boxGeo(0.1, 0.13, 0.1), cached('m:lantern', () => new THREE.MeshStandardMaterial({ color: 0xffe3a0, emissive: 0xffc968, emissiveIntensity: 0.9, roughness: 0.4 })));
  lantern.position.y = 1.3;
  const cap = mesh(geo('lamp-cap', () => new THREE.ConeGeometry(0.09, 0.07, 4)), std(0x3f3a36));
  cap.position.y = 1.43;
  cap.rotation.y = Math.PI / 4;
  g.add(post, lantern, cap);
  return shade(g);
}

// tall round table for standing breaks
export function standingTable() {
  const g = new THREE.Group();
  const top = cyl(0.3, 0.3, 0.04, 0xf2e9d8, 16);
  top.position.y = 0.62;
  const stem = cyl(0.03, 0.05, 0.6, 0x6b5a48, 8);
  stem.position.y = 0.3;
  const base = cyl(0.17, 0.17, 0.03, 0x6b5a48, 14);
  base.position.y = 0.015;
  g.add(top, stem, base);
  return g;
}

// post with a blank wooden board
export function sign() {
  const g = new THREE.Group();
  const post = cyl(0.025, 0.03, 0.8, 0x6b4a2e, 6);
  post.position.y = 0.4;
  const board = box(0.5, 0.26, 0.04, 0xc99a62);
  board.position.y = 0.72;
  g.add(post, board);
  return g;
}

// string of warm bulbs hanging between two points, sagging in the middle
export function stringLights(a, b, sag = 0.18, bulbs = 0) {
  const g = new THREE.Group();
  const len = Math.hypot(b.x - a.x, b.z - a.z);
  const n = bulbs || Math.max(4, Math.round(len / 0.22));
  const pts = [];
  for (let i = 0; i <= 24; i++) {
    const t = i / 24;
    pts.push(new THREE.Vector3(a.x + (b.x - a.x) * t, a.y + (b.y - a.y) * t - Math.sin(Math.PI * t) * sag, a.z + (b.z - a.z) * t));
  }
  g.add(new THREE.Line(track(new THREE.BufferGeometry().setFromPoints(pts)), cached('m:wire', () => new THREE.LineBasicMaterial({ color: 0x3a3028 }))));
  const inst = track(new THREE.InstancedMesh(geo('bulb', () => new THREE.SphereGeometry(0.022, 6, 5)), cached('m:bulb', () => new THREE.MeshBasicMaterial({ color: 0xffffff, toneMapped: false })), n));
  const m4 = new THREE.Matrix4();
  const cols = [0xfff0b8, 0xffc978, 0xfff0b8, 0xff9f7a];
  const c = new THREE.Color();
  for (let i = 0; i < n; i++) {
    const t = (i + 0.5) / n;
    m4.setPosition(a.x + (b.x - a.x) * t, a.y + (b.y - a.y) * t - Math.sin(Math.PI * t) * sag - 0.02, a.z + (b.z - a.z) * t);
    inst.setMatrixAt(i, m4);
    inst.setColorAt(i, c.set(cols[i % 4]));
  }
  g.add(inst);
  return g;
}

// oval pond with a stone rim, lily pads, reeds and a duck. returns the group and a tick(t) for the bobbing.
export function pond(rx, rz) {
  const g = new THREE.Group();
  const water = mesh(geo('disc40', () => new THREE.CircleGeometry(1, 40)), cached('m:pond', () => new THREE.MeshPhysicalMaterial({ color: 0x5db8d8, roughness: 0.08, metalness: 0.0, clearcoat: 0.6, envMapIntensity: 1.6 })));
  water.rotation.x = -Math.PI / 2;
  water.scale.set(rx, rz, 1);
  water.position.y = -0.045;
  water.receiveShadow = true;
  const bed = mesh(geo('disc40', () => new THREE.CircleGeometry(1, 40)), std(0xd9c79a));
  bed.rotation.x = -Math.PI / 2;
  bed.scale.set(rx + 0.16, rz + 0.16, 1);
  bed.position.y = -0.06;
  g.add(bed, water);
  const r = rng(77);
  const n = 26;
  for (let i = 0; i < n; i++) {
    const a = (i / n) * Math.PI * 2;
    const st = rock(0.7 + r() * 0.7, i);
    st.position.set(Math.cos(a) * (rx + 0.1), 0.02, Math.sin(a) * (rz + 0.1));
    g.add(st);
  }
  const pads = [];
  const padGeo = geo('pad', () => new THREE.CircleGeometry(0.13, 12, 0.3, Math.PI * 2 - 0.5));
  for (let i = 0; i < 9; i++) {
    const a = r() * 6.28;
    const k = 0.15 + r() * 0.6;
    const pad = mesh(padGeo, std(i % 2 ? 0x4f9a4f : 0x66ae58, { side: THREE.DoubleSide }));
    pad.rotation.set(-Math.PI / 2, 0, r() * 6);
    pad.scale.setScalar(0.8 + r() * 0.7);
    pad.position.set(Math.cos(a) * rx * k, -0.035, Math.sin(a) * rz * k);
    pad.receiveShadow = true;
    g.add(pad);
    pads.push(pad);
    if (i % 3 === 0) {
      const f = mesh(geo('lily-flower', () => new THREE.ConeGeometry(0.05, 0.06, 6)), flat(i === 0 ? 0xffffff : 0xf27fa0));
      f.position.set(pad.position.x + 0.02, -0.01, pad.position.z);
      g.add(f);
    }
  }
  for (let i = 0; i < 7; i++) {
    const a = 0.2 + i * 0.12;
    const hgt = 0.4 + (i % 3) * 0.08;
    const c = mesh(geo(`reed:${hgt.toFixed(2)}`, () => new THREE.ConeGeometry(0.014, hgt, 4)), flat(0x6aa64b));
    c.position.set(Math.cos(a + 3.0) * (rx - 0.15), 0.18, Math.sin(a + 3.0) * (rz - 0.1) + (i % 2) * 0.05);
    c.rotation.z = (i % 2 ? 1 : -1) * 0.1;
    g.add(c);
  }
  const duck = new THREE.Group();
  const body = mesh(geo('duck-body', () => new THREE.SphereGeometry(0.07, 10, 8)), flat(0xffe066));
  body.scale.set(1.3, 0.8, 1);
  const head = mesh(geo('duck-head', () => new THREE.SphereGeometry(0.04, 8, 6)), flat(0xffe066));
  head.position.set(0.09, 0.07, 0);
  const beak = mesh(geo('duck-beak', () => new THREE.ConeGeometry(0.018, 0.04, 5)), flat(0xf08a2b));
  beak.rotation.z = -Math.PI / 2;
  beak.position.set(0.14, 0.065, 0);
  duck.add(body, head, beak);
  duck.position.y = -0.01;
  g.add(shade(duck));
  const tick = (t) => {
    pads.forEach((p, i) => { p.position.y = -0.035 + Math.sin(t * 1.2 + i) * 0.004; });
    const a = t * 0.18;
    duck.position.x = Math.cos(a) * rx * 0.6;
    duck.position.z = Math.sin(a) * rz * 0.6;
    duck.rotation.y = -a - Math.PI / 2;
    duck.rotation.z = Math.sin(t * 2) * 0.05;
  };
  return { group: g, tick };
}

// stone fountain: basin, pedestal, bowl, water and a small spray. height about 0.5
export function fountain() {
  const g = new THREE.Group();
  const stone = flat(0xb9b2a6);
  const wat = cached('m:fountain-water', () => new THREE.MeshPhysicalMaterial({ color: 0x6cc4e0, roughness: 0.05, clearcoat: 0.6 }));
  const part = (key, make, m, y) => { const p = mesh(geo(key, make), m); p.position.y = y; g.add(p); };
  part('f-basin', () => new THREE.CylinderGeometry(0.3, 0.32, 0.16, 12), stone, 0.08);
  part('f-water', () => new THREE.CylinderGeometry(0.26, 0.26, 0.02, 12), wat, 0.155);
  part('f-ped', () => new THREE.CylinderGeometry(0.05, 0.07, 0.3, 8), stone, 0.3);
  part('f-bowl', () => new THREE.CylinderGeometry(0.15, 0.06, 0.07, 12), stone, 0.46);
  part('f-top', () => new THREE.CylinderGeometry(0.12, 0.12, 0.012, 12), wat, 0.497);
  shade(g);
  const dropGeo = geo('drop', () => new THREE.SphereGeometry(0.014, 6, 5));
  const dropMat = cached('m:drop', () => new THREE.MeshBasicMaterial({ color: 0xcdefff, toneMapped: false }));
  const drops = [];
  for (let i = 0; i < 6; i++) { const d = mesh(dropGeo, dropMat); g.add(d); drops.push(d); }
  g.userData.tick = (t) => drops.forEach((d, i) => {
    const p = (t * 0.8 + i / 6) % 1;
    const a = (i / 6) * 6.28;
    d.position.set(Math.cos(a) * p * 0.2, 0.5 + Math.sin(p * Math.PI) * 0.1 - p * 0.04, Math.sin(a) * p * 0.2);
  });
  return g;
}

export { at };
