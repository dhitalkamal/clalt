import * as THREE from 'three';
import { furniture } from './assets.js';
import { makeStool, glowSprite } from './props.js';
import { whiteboardTexture } from './art.js';
import { box, std, steamPuffs } from './details.js';
import { reviewPages, markerTray, coffeePot, cupsStack, sugarJar, scoreBoard, arcadeCabinet } from './small.js';
import { MEZ } from './layout.js';
import { board, flat, sign, SPACE_TINT, spareDesk } from './zones.js';

const metal = std(0x8a949c, { metalness: 0.4 });
const white = std(0xe9e1d2);

function menuTexture() {
  const cv = document.createElement('canvas');
  cv.width = 280; cv.height = 200;
  const c = cv.getContext('2d');
  c.fillStyle = '#2b2a2e'; c.fillRect(0, 0, 280, 200);
  c.fillStyle = '#f2e6cf'; c.font = 'bold 26px system-ui'; c.fillText('Coffee', 18, 36);
  c.font = '18px system-ui';
  [['espresso', '2.0'], ['flat white', '3.0'], ['cold brew', '3.5'], ['croissant', '2.5'], ['oat milk', '+0.5']].forEach(([n, p], i) => { c.fillText(n, 18, 70 + i * 26); c.fillText(p, 200, 70 + i * 26); });
  const t = new THREE.CanvasTexture(cv);
  t.colorSpace = THREE.SRGBColorSpace;
  return new THREE.Mesh(new THREE.PlaneGeometry(1, 1), new THREE.MeshBasicMaterial({ map: t, toneMapped: false }));
}

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
  const g = new THREE.Group();
  g.add(m);
  m.castShadow = true;
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

// thin white pergola over cafe and play area, pendants hang from its beams
// sign hung from the pergola front beam by two cords
function hang(b, sg, x, z) {
  const g = new THREE.Group();
  for (const dx of [-0.25, 0.25]) { const c = new THREE.Mesh(new THREE.CylinderGeometry(0.004, 0.004, 0.25, 4), std(0x222222)); c.position.set(dx, 1.82, 0); g.add(c); }
  sg.position.set(0, 1.62, 0);
  g.add(sg);
  b.put(g, x, 0, z, 0, null);
}

function pergola(L, b, hot) {
  const { put, g } = b;
  const x0 = L.cafe.wx0;
  const x1 = L.rec.wx1;
  const z0 = L.cafe.wz0;
  const z1 = L.cafe.wz1;
  const H = 1.95;
  const mat = std(0xffffff, { transparent: true, opacity: 0.55 });
  const H2 = H;
  const beam = (w, d, x, z, y) => { const m = new THREE.Mesh(new THREE.BoxGeometry(w, 0.04, d), mat); m.position.set(x, y, z); g.add(m); };
  for (const x of [x0, (x0 + x1) / 2, x1]) for (const z of [z0, z1]) {
    const p = new THREE.Mesh(new THREE.BoxGeometry(0.05, H, 0.05), mat);
    put(p, x, H / 2, z, 0, 'post', 'c5', { onFloor: true });
  }
  for (const z of [z0, z0 + 2.0, z1]) beam(x1 - x0, 0.05, (x0 + x1) / 2, z, H);
  for (const x of [x0, (x0 + x1) / 2, x1]) beam(0.05, z1 - z0, x, (z0 + z1) / 2, H);
  // pendants over the two high tables and the ping-pong table, hanging from the z0+2 beam
  const P = L.P;
  for (const [x, z] of [[P.table1.x, P.table1.z], [P.table2.x, P.table2.z], [P.pong.x, P.pong.z]]) {
    const pg = new THREE.Group();
    const cord = new THREE.Mesh(new THREE.CylinderGeometry(0.004, 0.004, 0.3, 4), std(0x222222));
    cord.position.y = H - 0.17;
    const shade = new THREE.Mesh(new THREE.ConeGeometry(0.09, 0.08, 14, 1, true), std(0xf2d9a8, { side: THREE.DoubleSide, emissive: 0xffc27a, emissiveIntensity: 0.6 }));
    shade.position.y = H - 0.36;
    const bulb = glowSprite(0xffd9a0, 0.22, 0.7);
    bulb.position.y = H - 0.4;
    pg.add(cord, shade, bulb);
    put(pg, x, 0, z, 0, null);
  }
  void hot;
}

function cafe(L, b, hot) {
  const { put, g, ticks } = b;
  const c = L.cafe;
  const x0 = c.wx0;
  const z0 = c.wz0;
  g.add(flat(4.0, 3.0, SPACE_TINT.cafe, 0.22));
  g.children[g.children.length - 1].position.set(c.cx, 0.007, c.wz0 + 1.5);
  put(furniture('rugRectangle', { scale: 1.9, noCast: true }), x0 + 2.0, 0.012, z0 + 1.75, 0, 'rug', 'c25');
  // counter modules along the back, 0.5 wide each
  const mods = [];
  [0.75, 1.25, 1.75].forEach((dx, i) => {
    const cab = furniture(i === 1 ? 'kitchenCabinetDrawer' : 'kitchenCabinet');
    const sc = 0.5 / cab.userData.size.x;
    cab.scale.setScalar(sc);
    const dep = cab.userData.size.z * sc;
    put(cab, x0 + dx, 0, z0 + 0.12 + dep / 2, Math.PI, 'counter', 'x25', { onFloor: true });
    mods.push(cab.userData.size.y * sc);
  });
  const bz = z0 + 0.35;
  const mach = furniture('kitchenCoffeeMachine', { scale: 1.2 });
  put(mach, x0 + 0.75, mods[0], bz, Math.PI, null);
  const st = steamAt(x0 + 0.75, mods[0] + 0.2, bz + 0.05, (t) => t < (L.hotRef.until || 0));
  g.add(st);
  ticks.push((t) => st.userData.tick(t));
  put(pastryCase(), x0 + 1.25, mods[1], bz, Math.PI, null);
  put(cupsStack(), x0 + 1.6, mods[2], bz, 0, null);
  put(sugarJar(), x0 + 1.9, mods[2], bz, 0, null);
  put(furniture('pottedPlant', { scale: 1.3 }), x0 + 0.25, 0, z0 + 0.25, 0, 'plant', 'c25', { onFloor: true });
  put(board(0.5, 0.7, 0.3, menuTexture(), true), x0 + 3.5, 0, z0 + 0.25, Math.PI, 'easel', 'c25', { onFloor: true });
  // high tables, stools tucked next to them
  for (const [tx, tz, stools] of [[L.P.table1.x, L.P.table1.z, [[0, -0.42], [0, 0.42]]], [L.P.table2.x, L.P.table2.z, [[0, -0.42], [0, 0.42]]]]) {
    const tb = furniture('tableRound');
    const sz = tb.userData.size;
    tb.scale.set(0.5 / sz.x, 0.55 / sz.y, 0.5 / sz.z);
    put(tb, tx, 0, tz, 0, 'table', 'c25', { onFloor: true });
    stools.forEach(([dx, dz]) => put(makeStool(0xb9825a, 2), tx + dx, 0, tz + dz, 0, 'chair', 'none', { onFloor: true }));
    const mug = new THREE.Mesh(new THREE.CylinderGeometry(0.018, 0.016, 0.04, 10), std(0xe9d9c0));
    put(mug, tx, 0.57, tz, 0, null);
    const sp = steamAt(tx, 0.6, tz, (t) => t < (L.hotRef.until || 0));
    g.add(sp); sp.position.set(tx, 0.6, tz);
    ticks.push((t) => sp.userData.tick(t));
  }
  hang(b, sign('CAFE', SPACE_TINT.cafe), c.cx, z0 + 3.0);
}

function rec(L, b) {
  const { put, g, ticks } = b;
  const r = L.rec;
  const P = L.P;
  g.add(flat(4.0, 3.0, SPACE_TINT.rec, 0.22));
  g.children[g.children.length - 1].position.set(r.cx, 0.007, r.wz0 + 1.5);
  put(furniture('rugRectangle', { scale: 1.8, noCast: true }), P.pong.x, 0.012, P.pong.z, 0, 'rug', 'c25');
  const pp = pingPong();
  put(pp.tbl, P.pong.x, 0, P.pong.z, 0, 'table', 'c25', { onFloor: true, name: 'pingpong' });
  L.pongBall = pp.ball;
  const sb = scoreBoard();
  sb.position.set(0, 0.355 + 0.06, -0.26);
  pp.tbl.add(sb);
  ticks.push((t) => sb.userData.tick(t));
  P.bags.forEach((p, i) => put(beanBag([0xd9534f, 0x4f86c6][i]), p.x, 0, p.z, 0, 'bean', 'c25', { onFloor: true }));
  P.arcades.forEach((p) => {
    const ar = arcadeCabinet();
    put(ar, p.x, 0, p.z, -Math.PI / 2, 'arcade', 'c25', { onFloor: true });
    ticks.push((t) => ar.userData.tick(t));
  });
  put(furniture('pottedPlant', { scale: 1.3 }), r.wx1 - 0.25, 0, r.wz1 - 0.25, 0, 'plant', 'c25', { onFloor: true });
  put(furniture('plantSmall2', { scale: 1.6 }), r.wx0 + 0.25, 0, r.wz1 - 0.25, 0, 'plant', 'c25', { onFloor: true });
  hang(b, sign('PLAY', SPACE_TINT.rec), r.cx, r.wz1);
}

function mezzanine(L, b) {
  const { put, g } = b;
  const d = L.deck;
  const w = d.x1 - d.x0;
  const dep = d.z1 - d.z0;
  const slab = box(w, 0.1, dep, 0xcaa06c);
  slab.position.set((d.x0 + d.x1) / 2, MEZ - 0.05, (d.z0 + d.z1) / 2);
  slab.receiveShadow = true;
  g.add(slab);
  const fascia = box(w, 0.06, 0.03, 0xf6f1e8);
  fascia.position.set((d.x0 + d.x1) / 2, MEZ - 0.13, d.z1);
  g.add(fascia);
  // posts every 1.5, aligned with the cafe and play area boundary below
  for (let k = 0; k <= 4; k++) {
    const p = box(0.07, MEZ - 0.1, 0.07, 0xe9e1d2);
    put(p, d.x0 + k * 2.0, (MEZ - 0.1) / 2, d.z1, 0, 'post', 'c5', { onFloor: true });
  }
  const rail = (x0, z0, x1, z1) => {
    const len = Math.hypot(x1 - x0, z1 - z0);
    const r = box(len, 0.03, 0.03, 0x8a949c);
    r.position.set((x0 + x1) / 2, MEZ + 0.34, (z0 + z1) / 2);
    r.rotation.y = -Math.atan2(z1 - z0, x1 - x0);
    g.add(r);
    const n = Math.max(2, Math.round(len / 0.25));
    for (let i = 0; i <= n; i++) {
      const bb = box(0.012, 0.34, 0.012, 0x8a949c);
      bb.position.set(x0 + ((x1 - x0) * i) / n, MEZ + 0.17, z0 + ((z1 - z0) * i) / n);
      g.add(bb);
    }
  };
  rail(d.x0, d.z1, d.x1, d.z1);
  rail(d.x0, d.z0, d.x0, d.z1);
  rail(d.x1, d.z0, d.x1, d.z1 - 1.0);
  // staircase flush with the deck's east edge: landing level with the slab, flight descending toward +z
  const sx = d.x1 + 0.5;
  const n = 8;
  const run = d.zb - d.zt;
  for (let i = 0; i < n; i++) {
    const t = (i + 1) / n;
    const step = box(1.0, 0.04, run / n, 0xd8c9ac);
    step.position.set(sx, MEZ * t - 0.02, d.zb - (run * (i + 0.5)) / n);
    g.add(step);
  }
  const landing = box(1.0, 0.1, 1.0, 0xcaa06c);
  landing.position.set(sx, MEZ - 0.05, d.z1 - 0.5);
  g.add(landing);
  for (const dx of [-0.5, 0.5]) {
    const len = Math.hypot(run, MEZ);
    const str = box(0.03, 0.04, len, 0x8a949c);
    str.position.set(sx + dx, MEZ / 2 + 0.3, (d.zb + d.zt) / 2);
    str.rotation.x = Math.atan2(MEZ, run);
    g.add(str);
    for (let i = 0; i <= 4; i++) {
      const bb = box(0.012, 0.3, 0.012, 0x8a949c);
      bb.position.set(sx + dx, MEZ * (1 - i / 4) + 0.15, d.zt + (run * i) / 4);
      g.add(bb);
    }
  }
  // review room: round table, rug, side table with printed pages and the LGTM sticky
  const rv = L.review;
  const tb = furniture('tableRound');
  const sz = tb.userData.size;
  tb.scale.set(1.1 / sz.x, 0.38 / sz.y, 1.1 / sz.z);
  put(tb, rv.table.x, MEZ, rv.table.z, 0, 'table', 'c5', { onFloor: true, y0: MEZ });
  put(furniture('rugRound', { scale: 2.0, noCast: true }), rv.table.x, MEZ + 0.012, rv.table.z, 0, 'rug', 'c5');
  const st = furniture('sideTable');
  put(st, d.x0 + 0.5, MEZ, d.z0 + 0.25, Math.PI, 'table', 'c25', { onFloor: true, y0: MEZ });
  put(reviewPages(), d.x0 + 0.5, MEZ + 0.38, d.z0 + 0.25, 0, null);
  put(furniture('pottedPlant', { scale: 1.3 }), d.x0 + 3.75, MEZ, d.z0 + 0.25, 0, 'plant', 'c25', { onFloor: true, y0: MEZ });
  put(sign('REVIEW', SPACE_TINT.review), rv.cx, MEZ + 0.46, d.z1, 0, null);
  // meeting room: whiteboard on an easel with a marker tray and diagram, rug under the desks, side table with a coffee pot
  const m = L.meeting;
  const wbFace = new THREE.Mesh(new THREE.PlaneGeometry(1, 1), new THREE.MeshBasicMaterial({ map: whiteboardTexture(), toneMapped: false }));
  put(board(1.3, 0.7, 0.3, wbFace, true), m.cx, MEZ, d.z0 + 0.5, Math.PI, 'easel', 'c25', { onFloor: true, y0: MEZ });
  put(markerTray(), m.cx, MEZ + 0.3, d.z0 + 0.5 + 0.04, Math.PI, null);
  put(furniture('rugRectangle', { scale: 2.3, noCast: true }), m.cx, MEZ + 0.012, d.z0 + 2.5, 0, 'rug', 'c25');
  const stb = furniture('sideTable');
  put(stb, m.wx1 - 0.5, MEZ, d.z0 + 0.25, Math.PI, 'table', 'c25', { onFloor: true, y0: MEZ });
  put(coffeePot(), m.wx1 - 0.5, MEZ + 0.38, d.z0 + 0.25, 0, null);
  put(furniture('pottedPlant', { scale: 1.3 }), m.wx0 + 0.25, MEZ, d.z0 + 0.25, 0, 'plant', 'c25', { onFloor: true, y0: MEZ });
  put(sign('MEETING', SPACE_TINT.meeting), m.cx, MEZ + 0.46, d.z1, 0, null);
  // spare meeting desks
  for (const idx of L.spareMeeting || []) spareDesk(b, L.X(0) + 0 + (d.x0 - L.X(0)) + 5.0 + (idx % 2) * 2.0, d.z0 + 1.5 + Math.floor(idx / 2) * 1.5, MEZ);
  // under the deck: two storage racks against the wall
  for (const dx of [0.5, 1.0]) put(box(0.4, 0.9, 0.3, 0x30343a), d.x0 + dx, 0.45, d.z0 + 0.25, 0, 'rack', 'c25', { onFloor: true });
}

export function buildAnnex(L, b) {
  L.hotRef = L.hotRef || { until: 0 };
  cafe(L, b);
  rec(L, b);
  pergola(L, b);
  mezzanine(L, b);
  return { ticks: [] };
}
