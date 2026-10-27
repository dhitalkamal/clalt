import * as THREE from 'three';
import { furniture } from './assets.js';
import { textSprite, makeStool } from './props.js';
import { cabinDetails, box } from './details.js';
import { glass } from './build.js';

export const SPACE_TINT = { research: 0x5b8fc9, workspace: 0xd9b26a, review: 0x9a6fc0, meeting: 0x5fb38a, lobby: 0xcfa070, cafe: 0xb9825a, play: 0x6fb9a8,
  cabins: 0x9aa7b4 };
export const woodMat = new THREE.MeshStandardMaterial({ color: 0x8a5a36, roughness: 0.8 });
const panelMat = new THREE.MeshStandardMaterial({ color: 0xf3ede2, roughness: 0.8 });
const CW = 2.0;
const CD = 2.0;

export function flat(w, d, color, opacity) {
  const m = new THREE.Mesh(new THREE.PlaneGeometry(w, d), new THREE.MeshStandardMaterial({ color, roughness: 1, transparent: true, opacity }));
  m.rotation.x = -Math.PI / 2;
  m.receiveShadow = true;
  return m;
}

export function sign(text, accent, w = 0.64) {
  const s = textSprite(text, accent, null);
  s.scale.set(w, w / 4, 1);
  return s;
}

// freestanding board on two legs, or hung when legs is false. face is any plane mesh, scaled to w x h.
export function board(w, h, y, face, legs = true) {
  const g = new THREE.Group();
  g.add(box(w + 0.06, h + 0.06, 0.03, 0x8a5a36));
  face.scale.set(w, h, 1);
  face.position.z = 0.02;
  g.add(face);
  if (legs) for (const dx of [-w / 2 + 0.05, w / 2 - 0.05]) { const leg = box(0.03, y, 0.03, 0x8a5a36); leg.position.set(dx, -y / 2 - h / 2, 0); g.add(leg); }
  g.position.y = y + h / 2;
  const wrap = new THREE.Group();
  wrap.add(g);
  return wrap;
}

// an unoccupied desk: same furniture as an agent's seat, monitor dark, so empty seats look intentional.
// side is the side of the monitor return, +1 east and -1 west.
export function spareDesk(side = 1, tint = 0x9aa7b4) {
  const g = new THREE.Group();
  const dh = 0.38;
  const desk = furniture('desk');
  desk.rotation.y = Math.PI;
  const stool = makeStool(tint, 1);
  stool.position.set(0, 0, -0.36);
  const ret = new THREE.Group();
  const top = box(0.4, 0.03, 0.6, 0xd9b48a);
  top.position.y = dh - 0.015;
  ret.add(top);
  for (const [lx, lz] of [[-0.17, -0.25], [0.17, -0.25], [-0.17, 0.25], [0.17, 0.25]]) { const l = box(0.03, dh - 0.03, 0.03, 0x8a949c); l.position.set(lx, (dh - 0.03) / 2, lz); ret.add(l); }
  ret.position.set(side * 0.565, 0, -0.2);
  const mon = new THREE.Group();
  mon.position.set(side * 0.565, dh, -0.25);
  mon.rotation.y = -side * 0.96;
  mon.scale.setScalar(1.05);
  mon.add(furniture('computerScreen'));
  const kb = furniture('computerKeyboard');
  kb.position.set(0, dh, -0.06);
  kb.rotation.y = Math.PI;
  const lamp = furniture('lampSquareTable', { scale: 1.1 });
  lamp.position.set(side * -0.28, dh, 0.12);
  g.add(desk, stool, ret, mon, kb, lamp);
  return g;
}

// glass cabin around one main agent, or an empty one when s is null. local frame: x east, z south, origin
// at the centre of the cabin. west and east say whether a neighbour shares that glass side.
export function cabin(s, tint, west, east) {
  const root = new THREE.Group();
  const shell = new THREE.Group();
  const gl = glass(tint);
  const H = 0.85;
  const add = (o, x, y, z, parent = shell) => { o.position.set(x, y, z); parent.add(o); return o; };
  const pane = (w, d, x, z) => add(new THREE.Mesh(new THREE.BoxGeometry(w, H, d), gl), x, H / 2, z);
  const rail = (w, d, x, z, y = H) => add(box(w, 0.03, d, 0x8a5a36), x, y, z);
  // solid back panel so whiteboard and window can hang on it, glass sides and front with a door gap
  add(new THREE.Mesh(new THREE.BoxGeometry(CW, 1.1, 0.02), panelMat), 0, 0.55, -CD / 2);
  rail(CW, 0.04, 0, -CD / 2, 1.1);
  if (!west) { pane(0.02, CD, -CW / 2, 0); rail(0.04, CD, -CW / 2, 0); }
  if (!east) { pane(0.02, CD, CW / 2, 0); rail(0.04, CD, CW / 2, 0); }
  const dx = 0.8;
  const gapL = dx - 0.2;
  const gapR = dx + 0.2;
  const zf = CD / 2;
  pane(gapL + CW / 2, 0.02, (gapL - CW / 2) / 2, zf); rail(gapL + CW / 2, 0.04, (gapL - CW / 2) / 2, zf);
  if (CW / 2 - gapR > 0.02) { pane(CW / 2 - gapR, 0.02, (gapR + CW / 2) / 2, zf); rail(CW / 2 - gapR, 0.04, (gapR + CW / 2) / 2, zf); }
  for (const x of [gapL, gapR]) add(box(0.05, 1.0, 0.05, 0x8a5a36), x, 0.5, zf);
  add(box(0.45, 0.05, 0.05, 0x8a5a36), dx, 1.0, zf);
  if (s) add(sign(s.project, tint, 0.55), dx - 0.2, 1.12, zf + 0.04, root);
  root.add(shell);
  const det = cabinDetails(s ? s.project : '', s ? s.seed : 0);
  root.add(det);
  const plant = furniture('pottedPlant', { scale: 1.1 });
  plant.position.set(-0.75, 0, 0.75);
  root.add(plant);
  if (!s) { const d = spareDesk(-1, tint); d.position.set(0.25, 0, 0.25); root.add(d); }
  return { root, plant };
}
