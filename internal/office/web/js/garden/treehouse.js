// raised platform for review (west half) and meeting (east half): log posts, plank deck, rope rails, a stair along
// the east side and eave awnings. the platform is 8.4 wide and sits TH_X off the footprint centre so the 1.1 wide
// stair fits the 9.6 footprint. chairs are not drawn here, the seat layer adds one per empty seat.
import * as THREE from 'three';
import { furniture } from '../assets.js';
import { MEZ, TH_X, TH_STAIR } from './local.js';
import { box, at, geo, cached } from './res.js';
import { plankTex, canvasTex } from './tex.js';
import { stringLights, bench, flowerBed } from './propmesh.js';
import { slab, awning } from './parts.js';

const LOG = 0x7a5232;
const ROPE = 0xd9c38f;

const logMat = () => cached('m:log', () => new THREE.MeshStandardMaterial({ color: LOG, roughness: 1, flatShading: true }));

function easel() {
  const g = new THREE.Group();
  const t = cached('t:easel', () => canvasTex(256, 140, (c, w, h) => {
    c.fillStyle = '#f7f6f1'; c.fillRect(0, 0, w, h);
    c.strokeStyle = '#2b5fa8'; c.lineWidth = 4; c.lineCap = 'round';
    c.beginPath(); c.moveTo(20, 30); c.lineTo(120, 30); c.moveTo(20, 60); c.lineTo(170, 60); c.moveTo(20, 90); c.lineTo(90, 90); c.stroke();
    c.strokeStyle = '#c0392b'; c.beginPath(); c.arc(200, 90, 24, 0, 7); c.stroke();
  }));
  g.add(at(box(1.3, 0.7, 0.04, 0x8a8f96), 0, 0.65, 0));
  g.add(at(new THREE.Mesh(geo('easel-face', () => new THREE.PlaneGeometry(1.2, 0.6)), cached('m:easel', () => new THREE.MeshBasicMaterial({ map: t, toneMapped: false }))), 0, 0.65, 0.025));
  for (const x of [-0.5, 0.5]) g.add(at(box(0.04, 0.3, 0.04, LOG), x, 0.15, 0));
  return g;
}

// a round table with a rug, scaled to the size the agents' monitors need
function roundTable(x, z, rug) {
  const g = new THREE.Group();
  const tb = furniture('tableRound');
  const sz = tb.userData.size;
  tb.scale.set(1.5 / sz.x, 0.38 / sz.y, 1.5 / sz.z);
  g.add(at(tb, x, MEZ, z));
  g.add(at(furniture(rug, { scale: 2.6, noCast: true }), x, MEZ + 0.012, z));
  return g;
}

export function treehouse() {
  const W = 8.4;
  const D = 4.2;
  const root = new THREE.Group();
  const g = new THREE.Group();
  g.position.x = TH_X;
  root.add(g);
  const x0 = -W / 2;
  const x1 = W / 2;
  const z0 = -D / 2;
  const z1 = D / 2;
  g.add(at(slab(W, D, plankTex(W, D, 28, 56), 0.16, 0x6a452a), 0, MEZ, 0));
  for (let k = 0; k <= 4; k++) {
    const x = x0 + k * 2.1;
    const front = new THREE.Mesh(geo(`log-front`, () => new THREE.CylinderGeometry(0.055, 0.065, MEZ + 0.85, 7)), logMat());
    front.castShadow = true;
    g.add(at(front, x, (MEZ + 0.85) / 2, z1 - 0.05));
    const back = new THREE.Mesh(geo(`log-back`, () => new THREE.CylinderGeometry(0.05, 0.06, MEZ, 7)), logMat());
    back.castShadow = true;
    g.add(at(back, x, MEZ / 2, z0 + 0.05));
  }
  for (let k = 0; k < 4; k++) {
    const len = Math.hypot(2.1, MEZ * 0.6);
    const br = box(len, 0.05, 0.05, LOG);
    br.position.set(x0 + k * 2.1 + 1.05, MEZ * 0.4, z1 - 0.05);
    br.rotation.z = (k % 2 ? 1 : -1) * Math.atan2(MEZ * 0.6, 2.1);
    g.add(br);
  }
  const rope = (ax, az, bx, bz) => {
    for (const y of [MEZ + 0.3, MEZ + 0.55]) {
      const r = box(Math.hypot(bx - ax, bz - az), 0.02, 0.02, ROPE);
      r.position.set((ax + bx) / 2, y, (az + bz) / 2);
      r.rotation.y = -Math.atan2(bz - az, bx - ax);
      g.add(r);
    }
  };
  rope(x0, z1 - 0.05, x1, z1 - 0.05);
  rope(x0, z0 + 0.05, x0, z1 - 0.05);
  rope(x1, z1 - 0.05, x1, 0.2);
  for (const z of [z0 + 0.05, 0]) g.add(at(box(0.07, 0.85, 0.07, LOG), x0, MEZ + 0.425, z));
  g.add(at(box(0.07, 0.85, 0.07, LOG), x1, MEZ + 0.425, 0.2));
  for (let k = 0; k < 4; k++) g.add(stringLights({ x: x0 + k * 2.1, y: MEZ + 0.82, z: z1 - 0.05 }, { x: x0 + (k + 1) * 2.1, y: MEZ + 0.82, z: z1 - 0.05 }, 0.16));
  // stair running north along the east edge, rising to deck level at the back corner
  const sx = TH_STAIR.x - TH_X;
  const n = 8;
  const zb = TH_STAIR.foot;
  const zt = TH_STAIR.top;
  const run = zb - zt;
  for (let i = 0; i < n; i++) {
    const t = (i + 1) / n;
    g.add(at(box(1.0, 0.05, run / n, 0xc9a06a), sx, MEZ * t - 0.025, zb - (run * (i + 0.5)) / n));
  }
  for (const dx of [-0.5, 0.5]) {
    const len = Math.hypot(run, MEZ);
    const str = box(0.04, 0.05, len, LOG);
    str.position.set(sx + dx, MEZ / 2 + 0.3, (zb + zt) / 2);
    str.rotation.x = Math.atan2(MEZ, run);
    g.add(str);
    for (let i = 0; i <= 4; i++) g.add(at(box(0.025, 0.3, 0.025, LOG), sx + dx, MEZ * (i / 4) + 0.15, zb - (run * i) / 4));
  }
  g.add(at(box(1.0, 0.04, 1.0, 0xc9a06a), sx, MEZ - 0.02, TH_STAIR.landing));
  // review (west) and meeting (east): round tables, rugs, side tables, plants, an easel
  g.add(roundTable(-2.1, 0.25, 'rugRound'), roundTable(2.1, 0.3, 'rugRectangle'));
  g.add(at(furniture('sideTable'), x0 + 0.5, MEZ, z0 + 0.3, Math.PI));
  g.add(at(furniture('pottedPlant', { scale: 1.3 }), -0.5, MEZ, z0 + 0.3));
  g.add(at(easel(), 2.1, MEZ, z0 + 0.6, Math.PI));
  g.add(at(furniture('sideTable'), x1 - 0.5, MEZ, -0.9, Math.PI));
  for (const [cx, tint] of [[-2.1, 0x9a6fc0], [2.1, 0x3a9d8f]]) {
    g.add(at(awning(4.0, 0.7, tint, 0.2), cx, MEZ + 0.98, z0 - 0.05));
    for (const dx of [-1.95, 1.95]) g.add(at(box(0.07, 1.0, 0.07, LOG), cx + dx, MEZ + 0.5, z0 + 0.05));
  }
  // under the deck: bench and flower boxes keep the shaded side friendly
  g.add(at(bench(), -2.4, 0, z1 - 0.5, Math.PI));
  g.add(at(flowerBed(1.2, 0.35, 15), -3.2, 0, z0 + 0.4), at(flowerBed(1.2, 0.35, 16), 2.2, 0, z0 + 0.4));
  return root;
}

