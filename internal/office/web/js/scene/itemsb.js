// item builders for the cafe, the play area and the lobby corner: the pieces that have a break spot or animate
import * as THREE from 'three';
import { furniture } from '../assets.js';
import { makeStool } from '../props.js';
import { box, std, steamPuffs } from '../details.js';
import { cupsStack, sugarJar, scoreBoard, arcadeCabinet, waterCooler } from '../small.js';
import { sway } from './itemsa.js';

// steam that shows only while hotFn says the coffee is fresh
function steamAt(x, y, z, hotFn) {
  const g = steamPuffs(0xffffff, 0.16);
  g.position.set(x, y, z);
  const tick = g.userData.tick;
  g.userData.tick = (t) => { g.visible = hotFn(t); if (g.visible) tick(t); };
  return g;
}

function pastryCase() {
  const g = new THREE.Group();
  g.add(box(0.4, 0.05, 0.22, 0xf6f1e8));
  const glass = new THREE.Mesh(new THREE.BoxGeometry(0.4, 0.14, 0.22), new THREE.MeshPhysicalMaterial({ color: 0xcfe8f5, transparent: true, opacity: 0.3, depthWrite: false }));
  glass.position.y = 0.1;
  g.add(glass);
  ['#d98a3d', '#f2c14b', '#c4574a', '#e9d9c0'].forEach((c, i) => { const m = box(0.07, 0.04, 0.07, c); m.position.set(-0.14 + i * 0.09, 0.06, i % 2 ? 0.04 : -0.04); g.add(m); });
  return g;
}

function beanBag(color) {
  const m = new THREE.Mesh(new THREE.SphereGeometry(0.3, 16, 12), std(color, { roughness: 0.9 }));
  m.scale.set(1, 0.5, 1);
  m.position.y = 0.15;
  m.castShadow = true;
  const g = new THREE.Group();
  g.add(m);
  return g;
}

function pingPong() {
  const tbl = new THREE.Group();
  const top = box(1.1, 0.03, 0.62, 0x2e7d6a);
  top.position.y = 0.34;
  const line = box(1.1, 0.032, 0.012, 0xf6f1e8);
  line.position.y = 0.34;
  const net = box(0.012, 0.09, 0.66, 0xf6f1e8);
  net.position.y = 0.405;
  tbl.add(top, line, net);
  for (const [lx, lz] of [[-0.48, -0.25], [0.48, -0.25], [-0.48, 0.25], [0.48, 0.25]]) { const l = box(0.04, 0.34, 0.04, 0x30343a); l.position.set(lx, 0.17, lz); tbl.add(l); }
  const ball = new THREE.Mesh(new THREE.SphereGeometry(0.02, 10, 8), std(0xffffff, { emissive: 0xffffff, emissiveIntensity: 0.5 }));
  ball.position.y = 0.45;
  tbl.add(ball);
  return { tbl, ball };
}

// thin white pergola, posts every 4 units along x and beams over the full area. local frame is the footprint
// centre of the 8 x 3 unit footprint.
function pergola() {
  const g = new THREE.Group();
  const H = 1.95;
  const mat = std(0xffffff, { transparent: true, opacity: 0.55 });
  const beam = (w, d, x, z) => { const m = new THREE.Mesh(new THREE.BoxGeometry(w, 0.04, d), mat); m.position.set(x, H, z); g.add(m); };
  for (const x of [-4, 0, 4]) for (const z of [-1.5, 1.5]) {
    const p = new THREE.Mesh(new THREE.BoxGeometry(0.05, H, 0.05), mat);
    p.position.set(x, H / 2, z);
    g.add(p);
  }
  for (const z of [-1.5, 0.5, 1.5]) beam(8, 0.05, 0, z);
  for (const x of [-4, 0, 4]) beam(0.05, 3, x, 0);
  return g;
}

export const BUILD_B = {
  'coffee-bar': (c) => {
    // counter modules along the back row, machine, pastry case and cups on top. the bar is 3 modules wide.
    const g = new THREE.Group();
    const mods = [];
    [-0.75, -0.25, 0.25].forEach((x, i) => {
      const cab = furniture(i === 1 ? 'kitchenCabinetDrawer' : 'kitchenCabinet');
      const sc = 0.5 / cab.userData.size.x;
      cab.scale.setScalar(sc);
      const dep = cab.userData.size.z * sc;
      cab.position.set(x, 0, 0.12 + dep / 2 - 0.5);
      cab.rotation.y = Math.PI;
      g.add(cab);
      mods.push(cab.userData.size.y * sc);
    });
    const bz = -0.15;
    const put = (o, x, y, z, ry = 0) => { o.position.set(x, y, z); o.rotation.y = ry; g.add(o); return o; };
    put(furniture('kitchenCoffeeMachine', { scale: 1.2 }), -0.75, mods[0], bz, Math.PI);
    const st = steamAt(-0.75, mods[0] + 0.2, bz + 0.05, (t) => t < (c.hot.until || 0));
    g.add(st);
    c.b.ticks.push((t) => st.userData.tick(t));
    put(pastryCase(), -0.25, mods[1], bz, Math.PI);
    put(cupsStack(), 0.1, mods[2], bz);
    put(sugarJar(), 0.4, mods[2], bz);
    return g;
  },
  'high-table': (c) => {
    const g = new THREE.Group();
    const tb = furniture('tableRound');
    const sz = tb.userData.size;
    tb.scale.set(0.5 / sz.x, 0.55 / sz.y, 0.5 / sz.z);
    g.add(tb);
    for (const dz of [-0.42, 0.42]) { const s = makeStool(0xb9825a, 2); s.position.set(0, 0, dz); g.add(s); }
    const mug = new THREE.Mesh(new THREE.CylinderGeometry(0.018, 0.016, 0.04, 10), std(0xe9d9c0));
    mug.position.y = 0.57;
    g.add(mug);
    const sp = steamAt(0, 0.6, 0, (t) => t < (c.hot.until || 0));
    g.add(sp);
    c.b.ticks.push((t) => sp.userData.tick(t));
    return g;
  },
  'ping-pong': (c) => {
    const pp = pingPong();
    const sb = scoreBoard();
    sb.position.set(0, 0.355 + 0.06, -0.26);
    pp.tbl.add(sb);
    c.b.ticks.push((t) => sb.userData.tick(t));
    c.b.pongs.push({ ball: pp.ball, a: c.id + ':a', b: c.id + ':b' });
    return pp.tbl;
  },
  arcade: (c) => {
    const ar = arcadeCabinet();
    ar.position.z = -0.25;
    c.b.ticks.push((t) => ar.userData.tick(t));
    const g = new THREE.Group();
    g.add(ar);
    return g;
  },
  'bean-bag-red': () => beanBag(0xd9534f),
  'bean-bag-blue': () => beanBag(0x4f86c6),
  'water-cooler': () => { const w = waterCooler(); w.position.z = -0.25; const g = new THREE.Group(); g.add(w); return g; },
  pergola,
};
export { sway };
