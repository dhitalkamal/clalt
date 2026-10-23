import * as THREE from 'three';
import { furniture } from './assets.js';
import { makeMug, glowSprite, glowTexture } from './props.js';

export const std = (color, extra = {}) => new THREE.MeshStandardMaterial({ color, roughness: 0.75, ...extra });
export const box = (w, h, d, color, extra) => { const m = new THREE.Mesh(new THREE.BoxGeometry(w, h, d), std(color, extra)); m.castShadow = true; return m; };
const STICKY = [0xffe27a, 0xff9fb2, 0x9fe0b5, 0x9ccaf2];

export function canvasTex(w, h, draw) {
  const cv = document.createElement('canvas');
  cv.width = w; cv.height = h;
  const c = cv.getContext('2d');
  draw(c, w, h);
  const t = new THREE.CanvasTexture(cv);
  t.colorSpace = THREE.SRGBColorSpace;
  t.anisotropy = 4;
  return { tex: t, ctx: c, w, h };
}

// steam: a few soft puffs that rise and fade, driven by tick(t)
export function steamPuffs(color = 0xffffff, rise = 0.12) {
  const g = new THREE.Group();
  const puffs = [0, 1, 2].map((i) => {
    const s = glowSprite(color, 0.06, 0.4);
    g.add(s);
    return { s, i };
  });
  g.userData.tick = (t) => {
    for (const { s, i } of puffs) {
      const p = (t * 0.45 + i / 3) % 1;
      s.position.set(Math.sin(t * 1.3 + i * 2) * 0.012, p * rise, 0);
      s.material.opacity = 0.42 * Math.sin(p * Math.PI);
      s.scale.setScalar(0.05 + p * 0.07);
    }
  };
  return g;
}

// everything small on one desk, varied per agent seed. holder frame: x across the desk, z toward the camera,
// the desk top spans x +-0.365 and z +-0.195, the typist sits at z = -0.36. keyboard sits at z -0.06.
export function deskDetails(seed, dh, round) {
  const g = new THREE.Group();
  const k = seed % 5;
  const mug = makeMug([0xe9d9c0, 0xc4574a, 0x5a8fbf, 0x7fb069, 0xf2b53c][k]);
  mug.position.set(-0.3, dh + 0.02, -0.12);
  g.add(mug);
  const steam = steamPuffs();
  steam.position.set(-0.3, dh + 0.05, -0.12);
  g.add(steam);
  // laptop on even desks, notebook and pen on odd ones, both square to the desk edge
  if (seed % 2 === 0 && !round) {
    const lap = furniture('laptop', { scale: 0.6 });
    lap.position.set(-0.24, dh, 0.08);
    g.add(lap);
  } else {
    const nb = box(0.1, 0.012, 0.11, [0x3d5a80, 0xa23b4a, 0x2f7d6a][seed % 3]);
    nb.position.set(-0.24, dh + 0.006, 0.08);
    const pen = box(0.012, 0.01, 0.1, 0x222222);
    pen.position.set(-0.16, dh + 0.005, 0.08);
    g.add(nb, pen);
  }
  // desk lamp on, warm pool on the desktop
  const lamp = furniture('lampSquareTable', { scale: 1.1 });
  lamp.position.set(0.28, dh, 0.12);
  g.add(lamp);
  const lampGlow = glowSprite(0xffcf8a, 0.5, 0.35);
  lampGlow.position.set(0.28, dh + 0.3, 0.12);
  g.add(lampGlow);
  const pool = new THREE.Mesh(new THREE.PlaneGeometry(0.5, 0.36), new THREE.MeshBasicMaterial({
    map: glowTexture(), color: 0xffc27a, transparent: true, opacity: 0.3, depthWrite: false, blending: THREE.AdditiveBlending, toneMapped: false }));
  pool.rotation.x = -Math.PI / 2;
  pool.position.set(0.1, dh + 0.004, 0.05);
  g.add(pool);
  // cable from a desk grommet to the keyboard
  const grom = new THREE.Mesh(new THREE.CylinderGeometry(0.022, 0.022, 0.006, 14), std(0x2a2d33));
  grom.position.set(0, dh + 0.003, 0.16);
  const cable = box(0.01, 0.008, 0.1, 0x1b1b1f);
  cable.position.set(0, dh + 0.004, 0.09);
  g.add(grom, cable);
  // one personal item next to the mouse
  const ix = 0.31;
  const iz = -0.12;
  if (k === 0) { const p = furniture('plantSmall1', { scale: 0.5 }); p.position.set(ix, dh, iz); g.add(p); }
  else if (k === 1) {
    const f = box(0.06, 0.07, 0.008, 0x7a5a3c); f.position.set(ix, dh + 0.035, iz);
    const ph = new THREE.Mesh(new THREE.PlaneGeometry(0.045, 0.05), new THREE.MeshBasicMaterial({ color: [0xffd3a0, 0xa0d3ff][seed % 2], toneMapped: false })); ph.position.set(ix, dh + 0.036, iz + 0.005); g.add(f, ph);
  } else if (k === 2) {
    const fig = new THREE.Mesh(new THREE.ConeGeometry(0.02, 0.06, 8), std(0xd95d5d)); fig.position.set(ix, dh + 0.03, iz);
    const head = new THREE.Mesh(new THREE.SphereGeometry(0.016, 8, 6), std(0xf2d3b0)); head.position.set(ix, dh + 0.07, iz); g.add(fig, head);
  } else if (k === 3) { const p = furniture('plantSmall2', { scale: 0.5 }); p.position.set(ix, dh, iz); g.add(p); }
  else { const toy = box(0.04, 0.04, 0.04, 0x4f86c6); toy.position.set(ix, dh + 0.02, iz); g.add(toy); }
  g.traverse((o) => { if (o.isMesh && !o.material.transparent) o.receiveShadow = true; });
  // desk mug steams only while the coffee is fresh: 40 s of every 120 s, offset per desk
  g.userData.tick = (t) => { const hot = ((t + seed * 23) % 120) < 40; steam.visible = hot; if (hot) steam.userData.tick(t); };
  return g;
}

// sticky notes on the monitor frame and headphones on top, in the monitor's own frame (screen toward +z)
export function monitorExtras(seed) {
  const g = new THREE.Group();
  for (let i = 0; i < 3; i++) {
    const n = new THREE.Mesh(new THREE.PlaneGeometry(0.035, 0.035), new THREE.MeshBasicMaterial({ color: STICKY[(seed + i) % 4], toneMapped: false }));
    n.position.set(0.215, 0.1 + i * 0.05, 0.054);
    g.add(n);
  }
  if (seed % 3 === 0) {
    const hp = new THREE.Group();
    const band = new THREE.Mesh(new THREE.TorusGeometry(0.05, 0.006, 5, 14, Math.PI), std(0x222831));
    for (const x of [-0.05, 0.05]) { const cup = new THREE.Mesh(new THREE.CylinderGeometry(0.02, 0.02, 0.015, 10), std(0x3b4350)); cup.rotation.z = Math.PI / 2; cup.position.set(x, -0.02, 0); hp.add(cup); }
    hp.add(band);
    hp.position.set(0, 0.315, 0);
    g.add(hp);
  }
  return g;
}

// cabin furnishing. local frame: x across (east +), z toward the door (south +), origin at the cabin centre, back panel at z = -0.75.
// the desk sits at x = +0.5, the rug is centred under desk and chair.
export function cabinDetails(project, seed) {
  const g = new THREE.Group();
  const rug = furniture('rugRectangle', { scale: 0.55, noCast: true });
  rug.position.set(0.35, 0.012, 0);
  g.add(rug);
  const shelf = furniture('bookcaseClosed', { scale: 1.2 });
  shelf.position.set(0.5, 0, -0.6);
  shelf.rotation.y = Math.PI;
  g.add(shelf);
  const books = furniture('books', { scale: 1.1 });
  books.position.set(0.5, 1.02, -0.6);
  g.add(books);
  // whiteboard with scribbles, a small window with a half-drawn blind, both on the back panel
  const wb = canvasTex(256, 160, (c, w, h) => {
    c.fillStyle = '#f7f6f1'; c.fillRect(0, 0, w, h);
    c.strokeStyle = ['#2b5fa8', '#c0392b', '#2f7d4f'][seed % 3]; c.lineWidth = 3; c.lineCap = 'round';
    c.beginPath(); c.moveTo(14, 30); c.lineTo(90, 30); c.moveTo(14, 55); c.lineTo(130, 55); c.moveTo(14, 80); c.lineTo(70, 80); c.stroke();
    c.beginPath(); c.arc(190, 60, 28, 0, 7); c.moveTo(190, 88); c.lineTo(190, 130); c.stroke();
    c.fillStyle = '#' + STICKY[seed % 4].toString(16).padStart(6, '0'); c.fillRect(20, 105, 40, 40);
  });
  const bframe = box(0.54, 0.35, 0.012, 0x8a8f96);
  bframe.position.set(-0.3, 0.6, -0.744);
  const board = new THREE.Mesh(new THREE.PlaneGeometry(0.5, 0.31), new THREE.MeshBasicMaterial({ map: wb.tex, toneMapped: false }));
  board.position.set(-0.3, 0.6, -0.736);
  board.rotation.y = 0;
  g.add(bframe, board);
  const win = box(0.3, 0.3, 0.01, 0xe7e1d5);
  win.position.set(-0.8, 0.62, -0.744);
  const sky = new THREE.Mesh(new THREE.PlaneGeometry(0.26, 0.26), new THREE.MeshBasicMaterial({ color: 0xbfe0f7, toneMapped: false }));
  sky.position.set(-0.8, 0.62, -0.737);
  const blind = new THREE.Mesh(new THREE.PlaneGeometry(0.26, 0.12), new THREE.MeshStandardMaterial({ color: 0xf1e6cf, roughness: 1 }));
  blind.position.set(-0.8, 0.69, -0.736);
  g.add(win, sky, blind);
  // coat rack in the back left corner, hat and coat
  const pole = box(0.02, 0.8, 0.02, 0x4a3b2a); pole.position.set(-0.88, 0.4, -0.6);
  const foot = new THREE.Mesh(new THREE.CylinderGeometry(0.07, 0.07, 0.015, 10), std(0x4a3b2a)); foot.position.set(-0.88, 0.008, -0.6);
  const coat = box(0.1, 0.22, 0.04, [0x3d5a80, 0x6b4a2f, 0x7a3b4a][seed % 3]); coat.position.set(-0.8, 0.6, -0.6);
  const hat = new THREE.Mesh(new THREE.ConeGeometry(0.04, 0.05, 10), std(0xc4574a)); hat.position.set(-0.88, 0.82, -0.6);
  g.add(pole, foot, coat, hat);
  return g;
}

// analog wall clock showing the real time, tick() redraws about once a second
export function wallClock() {
  const t = canvasTex(128, 128, () => {});
  const draw = () => {
    const c = t.ctx;
    const d = new Date();
    c.clearRect(0, 0, 128, 128);
    c.fillStyle = '#f7f3ea'; c.beginPath(); c.arc(64, 64, 60, 0, 7); c.fill();
    c.strokeStyle = '#3b3f46'; c.lineWidth = 6; c.stroke();
    c.lineWidth = 3;
    for (let i = 0; i < 12; i++) { const a = (i / 12) * Math.PI * 2; c.beginPath(); c.moveTo(64 + Math.sin(a) * 50, 64 - Math.cos(a) * 50); c.lineTo(64 + Math.sin(a) * 56, 64 - Math.cos(a) * 56); c.stroke(); }
    const hand = (a, len, w, col) => { c.strokeStyle = col; c.lineWidth = w; c.lineCap = 'round'; c.beginPath(); c.moveTo(64, 64); c.lineTo(64 + Math.sin(a) * len, 64 - Math.cos(a) * len); c.stroke(); };
    hand(((d.getHours() % 12) + d.getMinutes() / 60) / 12 * Math.PI * 2, 30, 5, '#222');
    hand((d.getMinutes() + d.getSeconds() / 60) / 60 * Math.PI * 2, 44, 3.5, '#222');
    hand(d.getSeconds() / 60 * Math.PI * 2, 48, 1.5, '#c0392b');
    t.tex.needsUpdate = true;
  };
  draw();
  const m = new THREE.Mesh(new THREE.PlaneGeometry(0.3, 0.3), new THREE.MeshBasicMaterial({ map: t.tex, transparent: true, toneMapped: false }));
  let last = 0;
  m.userData.tick = (now) => { if (now - last > 1) { last = now; draw(); } };
  return m;
}

export function noticeBoard(seed = 0) {
  const t = canvasTex(256, 160, (c, w, h) => {
    c.fillStyle = '#c99a62'; c.fillRect(0, 0, w, h);
    for (let i = 0; i < 90; i++) { c.fillStyle = `rgba(120,80,40,${0.1 + (i % 5) * 0.03})`; c.fillRect((i * 37) % w, (i * 53) % h, 3, 2); }
    const cols = ['#fff6c8', '#ffd0d8', '#cfe9ff', '#d6f5d6', '#ffffff'];
    for (let i = 0; i < 7; i++) {
      const x = 14 + (i % 4) * 58 + ((i * 13) % 9);
      const y = 12 + Math.floor(i / 4) * 70 + ((i * 7) % 11);
      c.save(); c.translate(x + 24, y + 28); c.rotate(((i * 31 + seed) % 7 - 3) * 0.04);
      c.fillStyle = cols[i % 5]; c.fillRect(-24, -28, 50, 58);
      c.fillStyle = '#d9534f'; c.beginPath(); c.arc(0, -24, 4, 0, 7); c.fill();
      c.fillStyle = '#555'; for (let l = 0; l < 4; l++) c.fillRect(-18, -14 + l * 11, 28 + (l * 7) % 12, 2);
      c.restore();
    }
  });
  const g = new THREE.Group();
  const frame = box(0.62, 0.42, 0.02, 0x7a5a3c);
  const face = new THREE.Mesh(new THREE.PlaneGeometry(0.56, 0.36), new THREE.MeshBasicMaterial({ map: t.tex, toneMapped: false }));
  face.position.z = 0.012;
  g.add(frame, face);
  return g;
}

// live call-graph screen for the research corner, tick() animates node pulses and a moving trace
export function liveGraphScreen() {
  const t = canvasTex(512, 320, () => {});
  const nodes = [[256, 70], [120, 140], [256, 150], [390, 140], [70, 230], [170, 235], [330, 232], [440, 230]];
  const edges = [[0, 1], [0, 2], [0, 3], [1, 4], [1, 5], [3, 6], [3, 7]];
  const draw = (now) => {
    const c = t.ctx;
    c.fillStyle = '#0a1220'; c.fillRect(0, 0, 512, 320);
    c.strokeStyle = '#2d4a74'; c.lineWidth = 3;
    edges.forEach(([a, b]) => { c.beginPath(); c.moveTo(...nodes[a]); c.lineTo(...nodes[b]); c.stroke(); });
    // pulses travelling along edges
    edges.forEach(([a, b], i) => {
      const p = (now * 0.5 + i * 0.37) % 1;
      c.fillStyle = '#ffd27a';
      c.beginPath(); c.arc(nodes[a][0] + (nodes[b][0] - nodes[a][0]) * p, nodes[a][1] + (nodes[b][1] - nodes[a][1]) * p, 5, 0, 7); c.fill();
    });
    nodes.forEach(([x, y], i) => {
      const pulse = 0.5 + 0.5 * Math.sin(now * 2 + i);
      c.fillStyle = `rgba(125,255,176,${0.25 + 0.25 * pulse})`; c.beginPath(); c.arc(x, y, 22 + pulse * 5, 0, 7); c.fill();
      c.fillStyle = i % 3 === 0 ? '#ffd27a' : '#7dffb0'; c.beginPath(); c.arc(x, y, 13, 0, 7); c.fill();
    });
    // sparkline along the bottom
    c.strokeStyle = '#7dffb0'; c.lineWidth = 2; c.beginPath();
    for (let x = 0; x < 512; x += 8) { const y = 300 - 12 - 10 * Math.sin(x * 0.04 + now * 3) - 6 * Math.sin(x * 0.13 - now * 2); x ? c.lineTo(x, y) : c.moveTo(x, y); }
    c.stroke();
    c.fillStyle = '#d6e8ff'; c.font = 'bold 20px ui-monospace, Menlo, monospace'; c.fillText('call graph - live', 14, 28);
    t.tex.needsUpdate = true;
  };
  draw(0);
  const m = new THREE.Mesh(new THREE.PlaneGeometry(1, 1), new THREE.MeshBasicMaterial({ map: t.tex, toneMapped: false }));
  let last = 0;
  m.userData.tick = (now) => { if (now - last > 0.12) { last = now; draw(now); } };
  return m;
}
