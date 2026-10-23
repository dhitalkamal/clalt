import * as THREE from 'three';
import { furniture } from './assets.js';
import { std, box, canvasTex } from './details.js';

const cyl = (rt, rb, h, color, seg = 14, extra) => { const m = new THREE.Mesh(new THREE.CylinderGeometry(rt, rb, h, seg), std(color, extra)); m.castShadow = true; return m; };
const hex = (n) => '#' + n.toString(16).padStart(6, '0');

// reception desk, long side along local x, front (with the sign-in screen) toward local +z. height 0.42.
export function receptionDesk() {
  const g = new THREE.Group();
  const body = box(1.0, 0.4, 0.5, 0xe8dcc6);
  body.position.y = 0.2;
  const top = box(1.04, 0.03, 0.54, 0x8a5a36);
  top.position.y = 0.415;
  g.add(body, top);
  const t = canvasTex(256, 128, (c, w, h) => {
    c.fillStyle = '#10162a'; c.fillRect(0, 0, w, h);
    c.fillStyle = '#ffd27a'; c.font = 'bold 30px system-ui'; c.fillText('Welcome', 20, 48);
    c.fillStyle = '#d6e8ff'; c.font = '22px system-ui'; c.fillText('sign in here', 20, 84);
    c.strokeStyle = '#7dffb0'; c.lineWidth = 3; c.strokeRect(150, 70, 90, 40);
  });
  const tablet = box(0.26, 0.17, 0.02, 0x30343a);
  tablet.position.set(-0.2, 0.52, 0.12);
  tablet.rotation.x = -0.3;
  const scr = new THREE.Mesh(new THREE.PlaneGeometry(0.23, 0.13), new THREE.MeshBasicMaterial({ map: t.tex, toneMapped: false }));
  scr.position.set(-0.2, 0.52, 0.132);
  scr.rotation.x = -0.3;
  const mon = furniture('computerScreen', { scale: 0.9 });
  mon.position.set(0.25, 0.43, -0.1);
  const bell = cyl(0.03, 0.035, 0.03, 0xd9b26a, 12, { metalness: 0.5 });
  bell.position.set(0.35, 0.445, 0.14);
  g.add(tablet, scr, mon, bell);
  return g;
}

export function umbrellaStand() {
  const g = new THREE.Group();
  const pot = cyl(0.1, 0.09, 0.26, 0x4a4f57);
  pot.position.y = 0.13;
  g.add(pot);
  [[-0.03, 0.02, 0xc4574a, 0.0], [0.03, -0.02, 0x3d5a80, 0.0], [0.0, 0.04, 0x2f7d6a, 0.0]].forEach(([x, z, c]) => {
    const u = cyl(0.008, 0.008, 0.36, 0x222222, 6);
    u.position.set(x, 0.3, z);
    const cone = new THREE.Mesh(new THREE.ConeGeometry(0.03, 0.12, 8), std(c));
    cone.position.set(x, 0.5, z);
    g.add(u, cone);
  });
  return g;
}

export function waterCooler() {
  const g = new THREE.Group();
  const body = box(0.22, 0.5, 0.22, 0xe9eef2);
  body.position.y = 0.25;
  const jug = cyl(0.08, 0.08, 0.2, 0x8fc7ee, 16, { transparent: true, opacity: 0.55 });
  jug.position.y = 0.6;
  const tap = box(0.03, 0.03, 0.04, 0x4f86c6);
  tap.position.set(0, 0.38, 0.12);
  g.add(body, jug, tap);
  return g;
}

// printer on a table, a sheet slides out when print() is called. tick(t) drives it.
export function printer() {
  const g = new THREE.Group();
  const body = box(0.28, 0.14, 0.22, 0xdadde2);
  body.position.y = 0.07;
  const lid = box(0.26, 0.015, 0.2, 0xb9bec5);
  lid.position.y = 0.147;
  const tray = box(0.18, 0.008, 0.02, 0xb9bec5);
  tray.position.set(0, 0.04, 0.12);
  const sheet = box(0.16, 0.004, 0.12, 0xffffff);
  sheet.position.set(0, 0.046, 0.1);
  g.add(body, lid, tray, sheet);
  const st = { start: -99 };
  g.userData.print = (now) => { st.start = now; };
  g.userData.tick = (now) => {
    const u = now - st.start;
    // a sheet rests in the tray; while printing it is pushed out and replaced
    const e = u >= 0 && u < 2.4 ? Math.min(1, u / 1.8) : 0;
    sheet.position.z = 0.1 + 0.05 * e;
    sheet.position.y = 0.046 + 0.002 * (u >= 2.4 || u < 0 ? 0 : 1);
  };
  return g;
}

// cork board with pinned notes and red strings between the pins, hung on a wall (faces local +z)
export function pinFace() {
  const t = canvasTex(320, 200, (c, W, H) => {
    c.fillStyle = '#c99a62'; c.fillRect(0, 0, W, H);
    for (let i = 0; i < 120; i++) { c.fillStyle = `rgba(120,80,40,${0.08 + (i % 5) * 0.03})`; c.fillRect((i * 37) % W, (i * 53) % H, 3, 2); }
    const pins = [];
    const cols = ['#fff6c8', '#ffd0d8', '#cfe9ff', '#d6f5d6'];
    for (let i = 0; i < 6; i++) {
      const x = 24 + (i % 3) * 100;
      const y = 18 + Math.floor(i / 3) * 90;
      c.fillStyle = cols[i % 4]; c.fillRect(x, y, 70, 62);
      c.fillStyle = '#666'; for (let l = 0; l < 4; l++) c.fillRect(x + 8, y + 18 + l * 10, 34 + (l * 9) % 18, 2);
      pins.push([x + 35, y + 6]);
      c.fillStyle = '#d9534f'; c.beginPath(); c.arc(x + 35, y + 6, 4, 0, 7); c.fill();
    }
    c.strokeStyle = '#b3262a'; c.lineWidth = 2;
    [[0, 1], [1, 2], [0, 4], [4, 5], [2, 5], [3, 4]].forEach(([a, b]) => { c.beginPath(); c.moveTo(...pins[a]); c.lineTo(...pins[b]); c.stroke(); });
  });
  return new THREE.Mesh(new THREE.PlaneGeometry(1, 1), new THREE.MeshBasicMaterial({ map: t.tex, toneMapped: false }));
}

export function bookStack() {
  const g = new THREE.Group();
  [[0.16, 0.03, 0.11, 0x3d5a80], [0.14, 0.028, 0.1, 0xa23b4a], [0.15, 0.025, 0.1, 0x2f7d6a], [0.12, 0.022, 0.09, 0xd9b26a]].reduce((y, [w, h, d, c]) => {
    const b = box(w, h, d, c);
    b.position.y = y + h / 2;
    g.add(b);
    return y + h;
  }, 0);
  return g;
}

export function magnifier() {
  const g = new THREE.Group();
  const ring = new THREE.Mesh(new THREE.TorusGeometry(0.04, 0.005, 6, 18), std(0x3b3f46, { metalness: 0.5 }));
  ring.rotation.x = Math.PI / 2;
  ring.position.set(0, 0.008, 0);
  const lens = new THREE.Mesh(new THREE.CircleGeometry(0.04, 18), new THREE.MeshPhysicalMaterial({ color: 0xcfe8f5, transparent: true, opacity: 0.4 }));
  lens.rotation.x = -Math.PI / 2;
  lens.position.y = 0.008;
  const handle = box(0.012, 0.012, 0.07, 0x6b4a2f);
  handle.position.set(0, 0.008, 0.075);
  g.add(ring, lens, handle);
  return g;
}

// printed review pages with red marks, one LGTM sticky
export function reviewPages() {
  const g = new THREE.Group();
  const t = canvasTex(128, 160, (c, w, h) => {
    c.fillStyle = '#fbfaf6'; c.fillRect(0, 0, w, h);
    c.fillStyle = '#888'; for (let i = 0; i < 11; i++) c.fillRect(10, 14 + i * 12, 60 + (i * 17) % 50, 3);
    c.strokeStyle = '#c0392b'; c.lineWidth = 3;
    c.beginPath(); c.moveTo(10, 40); c.lineTo(110, 40); c.moveTo(10, 90); c.lineTo(90, 96); c.stroke();
    c.beginPath(); c.arc(95, 66, 14, 0, 7); c.stroke();
  });
  for (let i = 0; i < 2; i++) {
    const p = new THREE.Mesh(new THREE.BoxGeometry(0.16, 0.004, 0.2), [std(0xffffff), std(0xffffff), new THREE.MeshBasicMaterial({ map: t.tex, toneMapped: false }), std(0xffffff), std(0xffffff), std(0xffffff)]);
    p.position.set(i * 0.2 - 0.1, 0.002 + i * 0.004, 0);
    g.add(p);
  }
  const s = canvasTex(64, 64, (c) => { c.fillStyle = '#9fe0b5'; c.fillRect(0, 0, 64, 64); c.fillStyle = '#1f5f3a'; c.font = 'bold 17px system-ui'; c.fillText('LGTM', 8, 38); });
  const sticky = new THREE.Mesh(new THREE.PlaneGeometry(0.06, 0.06), new THREE.MeshBasicMaterial({ map: s.tex, toneMapped: false }));
  sticky.rotation.x = -Math.PI / 2;
  sticky.position.set(0.12, 0.009, 0.0);
  g.add(sticky);
  return g;
}

export function markerTray() {
  const g = new THREE.Group();
  g.add(box(0.3, 0.015, 0.05, 0x8a8f96));
  [0xc0392b, 0x2b5fa8, 0x2f7d4f, 0x222222].forEach((c, i) => {
    const m = cyl(0.007, 0.007, 0.1, c, 8);
    m.rotation.z = Math.PI / 2;
    m.position.set(-0.09 + i * 0.06, 0.017, 0);
    g.add(m);
  });
  return g;
}

export function coffeePot() {
  const g = new THREE.Group();
  g.add(box(0.12, 0.02, 0.12, 0x30343a));
  const jug = cyl(0.04, 0.04, 0.12, 0xcfe8f5, 14, { transparent: true, opacity: 0.55 });
  jug.position.y = 0.08;
  const fill = cyl(0.035, 0.035, 0.06, 0x3b2a1e, 14);
  fill.position.y = 0.06;
  const handle = box(0.012, 0.08, 0.012, 0x222222);
  handle.position.set(0.055, 0.08, 0);
  g.add(jug, fill, handle);
  return g;
}

export function cupsStack() {
  const g = new THREE.Group();
  for (let i = 0; i < 4; i++) { const c = cyl(0.026, 0.02, 0.035, 0xf6f1e8, 12); c.position.y = 0.0175 + i * 0.018; g.add(c); }
  return g;
}

export function sugarJar() {
  const g = new THREE.Group();
  const j = cyl(0.03, 0.03, 0.06, 0xdfeaf2, 12, { transparent: true, opacity: 0.7 });
  j.position.y = 0.03;
  const s = cyl(0.026, 0.026, 0.04, 0xffffff, 12);
  s.position.y = 0.022;
  const lid = cyl(0.03, 0.03, 0.01, 0x8a8f96, 12);
  lid.position.y = 0.065;
  g.add(j, s, lid);
  return g;
}

// flip score counter for the ping-pong table, tick(t) changes the score every few seconds
export function scoreBoard() {
  const t = canvasTex(128, 64, () => {});
  const draw = (a, b) => {
    const c = t.ctx;
    c.fillStyle = '#10162a'; c.fillRect(0, 0, 128, 64);
    c.fillStyle = '#7dffb0'; c.font = 'bold 40px ui-monospace, Menlo, monospace'; c.fillText(String(a).padStart(2, '0'), 8, 46);
    c.fillStyle = '#ffd27a'; c.fillText(String(b).padStart(2, '0'), 72, 46);
    t.tex.needsUpdate = true;
  };
  draw(0, 0);
  const g = new THREE.Group();
  g.add(box(0.22, 0.12, 0.02, 0x30343a));
  const face = new THREE.Mesh(new THREE.PlaneGeometry(0.2, 0.1), new THREE.MeshBasicMaterial({ map: t.tex, toneMapped: false }));
  face.position.z = 0.011;
  g.add(face);
  let last = -1;
  g.userData.tick = (now) => { const n = Math.floor(now / 6); if (n !== last) { last = n; draw((n * 3) % 11, (n * 5 + 2) % 11); } };
  return g;
}

// arcade cabinet with marquee text and a flickering screen. faces local +z.
export function arcadeCabinet() {
  const g = new THREE.Group();
  const body = box(0.34, 0.72, 0.3, 0x2b2f6b);
  body.position.y = 0.36;
  const screen = canvasTex(64, 64, () => {});
  const drawScreen = (f) => {
    const c = screen.ctx;
    c.fillStyle = f ? '#1a2244' : '#10162a'; c.fillRect(0, 0, 64, 64);
    c.fillStyle = '#ffd27a'; c.fillRect(10 + (f ? 8 : 0), 40, 24, 6);
    c.fillStyle = '#7dffb0'; for (let i = 0; i < 6; i++) c.fillRect(6 + i * 9, 8 + ((i + f) % 2) * 6, 6, 6);
    c.fillStyle = '#ff6b6b'; c.fillRect(40, 44 - f * 6, 8, 8);
    screen.tex.needsUpdate = true;
  };
  drawScreen(0);
  const scr = new THREE.Mesh(new THREE.PlaneGeometry(0.25, 0.22), new THREE.MeshBasicMaterial({ map: screen.tex, toneMapped: false }));
  scr.position.set(0, 0.5, 0.155);
  const mq = canvasTex(128, 32, (c, w, h) => { c.fillStyle = '#ffd27a'; c.fillRect(0, 0, w, h); c.fillStyle = '#2b2f6b'; c.font = 'bold 12px ui-monospace, Menlo, monospace'; c.fillText('HI-SCORE 48200', 8, 14); c.fillText('INSERT COIN', 8, 28); });
  const marquee = new THREE.Mesh(new THREE.PlaneGeometry(0.3, 0.075), new THREE.MeshBasicMaterial({ map: mq.tex, toneMapped: false }));
  marquee.position.set(0, 0.68, 0.155);
  const panel = box(0.34, 0.04, 0.16, 0xf2b53c);
  panel.position.set(0, 0.28, 0.2);
  g.add(body, scr, marquee, panel);
  let last = -1;
  g.userData.tick = (now) => { const n = Math.floor(now * 4) % 2; if (n !== last) { last = n; drawScreen(n); } };
  return g;
}
