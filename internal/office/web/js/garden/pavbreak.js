// the cafe kiosk and the play lawn. break spots (counter, tables, ping-pong, bean bags) match local.js.
import * as THREE from 'three';
import { furniture } from '../assets.js';
import { std, box, cyl, at, geo, cached, flat } from './res.js';
import { stripeTex, checkTex } from './tex.js';
import { stringLights, lampPost } from './propmesh.js';
import { plaza, awning, post } from './parts.js';
import { sizeOf } from './footprints.js';
import { CAFE_TABLES, BEANBAGS, PONG_TABLE } from './local.js';

function umbrella(color, h = 1.15) {
  const g = new THREE.Group();
  g.add(at(cyl(0.015, 0.015, h, 0xe8e0d0, 6), 0, h / 2, 0));
  const cone = new THREE.Mesh(geo('umbrella', () => new THREE.ConeGeometry(0.62, 0.22, 16, 1, true)), std(0xffffff, { map: stripeTex(color, 8), side: THREE.DoubleSide }));
  cone.position.y = h + 0.02;
  cone.castShadow = true;
  g.add(cone);
  return g;
}

// low table with an umbrella and three low stools. the two on the x axis are the sipping spots
function cafeTable(color) {
  const g = new THREE.Group();
  g.add(at(cyl(0.3, 0.3, 0.03, 0xf2e9d8, 16), 0, 0.3, 0));
  g.add(at(cyl(0.03, 0.04, 0.3, 0x6b5a48, 8), 0, 0.15, 0));
  g.add(umbrella(color));
  for (const [x, z] of [[-0.5, 0], [0.5, 0], [0, 0.5]]) g.add(at(cyl(0.13, 0.13, 0.12, 0xb98a5e, 14), x, 0.08, z));
  return g;
}

export function cafe(p) {
  const { w: W, d: D } = sizeOf(p);
  const g = new THREE.Group();
  g.add(plaza(W, D));
  const cz = -1.3;
  g.add(at(box(2.6, 0.4, 0.5, 0xf2e9d8), -0.5, 0.2, cz));
  g.add(at(box(2.7, 0.03, 0.56, 0x8a5a36), -0.5, 0.415, cz));
  g.add(at(furniture('kitchenCoffeeMachine', { scale: 1.3 }), -1.2, 0.43, cz - 0.05));
  for (const x of [-0.2, 0.1, 0.4]) g.add(at(cyl(0.025, 0.02, 0.05, 0xffffff, 8), x, 0.455, cz + 0.05));
  for (const x of [-1.85, 0.85]) for (const dz of [-0.35, 0.45]) g.add(at(post(1.25, 0.06), x, 0, cz + dz));
  g.add(at(awning(2.8, 1.0, 0xe8647c, 0.18), -0.5, 1.28, cz + 0.05));
  g.add(at(cafeTable(0x5fd0c0), CAFE_TABLES[0][0], 0, CAFE_TABLES[0][1]), at(cafeTable(0xe8647c), CAFE_TABLES[1][0], 0, CAFE_TABLES[1][1]));
  g.add(at(lampPost(), W / 2 - 0.3, 0, -D / 2 + 0.3));
  g.add(stringLights({ x: 0.85, y: 1.25, z: cz + 0.45 }, { x: W / 2 - 0.3, y: 1.35, z: -D / 2 + 0.3 }, 0.14));
  g.add(stringLights({ x: -1.85, y: 1.25, z: cz + 0.45 }, { x: -W / 2 + 0.3, y: 1.4, z: D / 2 - 0.4 }, 0.16));
  g.add(at(lampPost(), -W / 2 + 0.3, 0, D / 2 - 0.4));
  g.add(at(furniture('pottedPlant', { scale: 1.3 }), W / 2 - 0.35, 0, D / 2 - 0.35));
  return g;
}

function pingpong() {
  const g = new THREE.Group();
  const H = 0.34;
  g.add(at(box(1.6, 0.04, 0.9, 0x3f9e6b), 0, H, 0));
  g.add(at(box(1.6, 0.045, 0.02, 0xffffff), 0, H + 0.001, 0));
  g.add(at(box(0.02, 0.045, 0.9, 0xffffff), 0, H + 0.001, 0));
  g.add(at(box(0.012, 0.1, 0.96, 0xf4f4f4), 0, H + 0.07, 0));
  for (const x of [-0.7, 0.7]) for (const z of [-0.36, 0.36]) g.add(at(box(0.05, H, 0.05, 0x444a50), x, H / 2, z));
  const ball = new THREE.Mesh(geo('pingpong-ball', () => new THREE.SphereGeometry(0.015, 8, 6)), cached('m:pingpong-ball', () => new THREE.MeshBasicMaterial({ color: 0xffffff })));
  ball.position.set(0.2, H + 0.12, 0.1);
  g.add(ball);
  g.userData.ball = ball;
  return g;
}

function beanbag(color) {
  const m = new THREE.Mesh(geo('beanbag', () => new THREE.SphereGeometry(0.3, 12, 8)), flat(color));
  m.scale.set(1, 0.5, 1);
  m.position.y = 0.14;
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

// play lawn: ping-pong table, picnic rug, bean bags. returns the group and a tick for the ball
export function play() {
  const g = new THREE.Group();
  const rug = new THREE.Mesh(geo('picnic', () => new THREE.PlaneGeometry(2.2, 1.6)), std(0xffffff, { map: checkTex(), roughness: 1 }));
  rug.rotation.x = -Math.PI / 2;
  rug.position.y = 0.012;
  rug.receiveShadow = true;
  g.add(at(rug, -1.2, 0.012, 0.9));
  const colors = [0xe8647c, 0x6bb6ee, 0xf2c14b];
  BEANBAGS.forEach(([x, z], i) => g.add(at(beanbag(colors[i % 3]), x, 0, z, 0.3)));
  const pp = pingpong();
  g.add(at(pp, PONG_TABLE[0], 0, PONG_TABLE[1]));
  g.add(at(post(0.9, 0.06), -1.2, 0, -1.4));
  const ball = pp.userData.ball;
  return { group: g, tick: (t) => ball.position.set(Math.sin(t * 2.8) * 0.7, 0.42 + Math.abs(Math.cos(t * 2.8)) * 0.14, 0.1) };
}

