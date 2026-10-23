import * as THREE from 'three';

export const SKY = 0xe6eff3;
// outdoor materials skip tone mapping so the far ground fades to exactly the sky clear color
// colors are pre-darkened because tone mapping is off and the sun is strong
const mat = (color, extra = {}) => new THREE.MeshStandardMaterial({ color: new THREE.Color(color).multiplyScalar(0.5), roughness: 1, toneMapped: false, ...extra });
const GREENS = [0x8fc27a, 0x7fb56d, 0x9ccb82];

function tree(x, z, s, i) {
  const g = new THREE.Group();
  const trunk = new THREE.Mesh(new THREE.CylinderGeometry(0.07 * s, 0.1 * s, 0.7 * s, 6), mat(0x9a7452));
  trunk.position.y = 0.35 * s;
  const top = new THREE.Mesh(new THREE.IcosahedronGeometry(0.5 * s, 0), mat(GREENS[i % 3]));
  top.position.y = 1.05 * s;
  top.scale.y = 1.2;
  const top2 = new THREE.Mesh(new THREE.IcosahedronGeometry(0.34 * s, 0), mat(GREENS[(i + 1) % 3]));
  top2.position.set(0.25 * s, 1.5 * s, 0.05 * s);
  g.add(trunk, top, top2);
  g.position.set(x, -0.08, z);
  g.rotation.y = i * 1.3;
  return g;
}

function bench(x, z) {
  const g = new THREE.Group();
  const wood = mat(0xb98a5e);
  const seat = new THREE.Mesh(new THREE.BoxGeometry(0.9, 0.05, 0.28), wood);
  seat.position.y = 0.28;
  const back = new THREE.Mesh(new THREE.BoxGeometry(0.9, 0.22, 0.04), wood);
  back.position.set(0, 0.45, -0.12);
  g.add(seat, back);
  for (const dx of [-0.38, 0.38]) { const l = new THREE.Mesh(new THREE.BoxGeometry(0.05, 0.28, 0.24), mat(0x6b7777)); l.position.set(dx, 0.14, 0); g.add(l); }
  g.position.set(x, -0.05, z);
  g.rotation.y = Math.PI / 2;
  g.traverse((o) => { if (o.isMesh) { o.castShadow = true; o.receiveShadow = true; } });
  return g;
}

// grass, a stone plaza around the office, a path to the lobby door, trees and a bench
export function buildGround(layout) {
  const { W, D, xmin, zmin, door } = layout;
  const g = new THREE.Group();
  const grass = new THREE.Mesh(new THREE.PlaneGeometry(400, 400), mat(0xa9d196));
  grass.rotation.x = -Math.PI / 2;
  grass.position.y = -0.1;
  grass.receiveShadow = true;
  g.add(grass);
  const px0 = xmin - 3.2;
  const px1 = xmin + W + 2.6;
  const pz0 = zmin - 2.6;
  const pz1 = zmin + D + 2.6;
  const plaza = new THREE.Mesh(new THREE.BoxGeometry(px1 - px0, 0.05, pz1 - pz0), mat(0xfff6e4));
  plaza.position.set((px0 + px1) / 2, -0.075, (pz0 + pz1) / 2);
  plaza.receiveShadow = true;
  g.add(plaza);
  const pathLen = 14;
  const path = new THREE.Mesh(new THREE.BoxGeometry(pathLen, 0.05, 0.9), mat(0xf0e4cc));
  path.position.set(px0 - pathLen / 2 + 1, -0.07, door.z);
  path.receiveShadow = true;
  g.add(path);
  g.add(bench(px0 + 1.6, door.z - 1.5));
  const pts = [];
  for (let i = 0; i < 10; i++) pts.push([px0 + 0.5 + i * ((px1 - px0 + 6) / 9) - 3, pz0 - 2.2 - (i % 3) * 1.6]);
  for (let i = 0; i < 6; i++) pts.push([px0 - 2.6 - (i % 2) * 1.8, pz0 + 1 + i * ((pz1 - pz0) / 5)]);
  for (let i = 0; i < 6; i++) pts.push([px1 + 1.8 + (i % 2) * 1.8, pz0 + 1 + i * ((pz1 - pz0) / 5)]);
  pts.forEach(([x, z], i) => {
    if (Math.abs(z - door.z) < 1.3 && x < px0) return; // keep the path clear
    g.add(tree(x, z, 1 + (i % 4) * 0.18, i));
  });
  g.traverse((o) => { if (o.isMesh && !o.userData.keep) o.castShadow = o !== grass && o !== plaza && o !== path; });
  return g;
}
