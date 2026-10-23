import * as THREE from 'three';
import { furniture } from './assets.js';
import { textSprite } from './props.js';
import { cabinDetails, wallClock, noticeBoard, liveGraphScreen, box, std } from './details.js';
import { receptionDesk, umbrellaStand, waterCooler, printer, pinFace, bookStack, magnifier } from './small.js';
import { ACCOUNT_TINT } from './model.js';
import { CW, CD, ROW } from './layout.js';
import { makeBuilder, glass } from './build.js';
import { buildAnnex } from './annex.js';
import { makeStool } from './props.js';

export const SPACE_TINT = { research: 0x5b8fc9, open: 0xd9b26a, review: 0x9a6fc0, meeting: 0x5fb38a, lobby: 0xcfa070, cafe: 0xb9825a, rec: 0x6fb9a8 };
export const woodMat = new THREE.MeshStandardMaterial({ color: 0x8a5a36, roughness: 0.8 });
const panelMat = new THREE.MeshStandardMaterial({ color: 0xf3ede2, roughness: 0.8 });

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

// glass cabin around one main agent. local frame: x east, z south, origin at the centre.
function cabin(s, hasWestNeighbour, hasEastNeighbour, b) {
  const c = s.cabin;
  const tint = ACCOUNT_TINT[s.account];
  const root = new THREE.Group();
  const shell = new THREE.Group();
  const gl = glass(tint);
  const H = 0.85;
  const add = (o, x, y, z, parent = shell) => { o.position.set(x, y, z); parent.add(o); return o; };
  const pane = (w, d, x, z) => { const m = add(new THREE.Mesh(new THREE.BoxGeometry(w, H, d), gl), x, H / 2, z); track0(m, 'wall'); return m; };
  const rail = (w, d, x, z, y = H) => add(box(w, 0.03, d, 0x8a5a36), x, y, z);
  const track0 = (m, kind) => b.track(m, kind);
  // solid back panel so whiteboard and window can hang on it, glass sides and front with a door gap
  const back = add(new THREE.Mesh(new THREE.BoxGeometry(CW, 1.1, 0.02), panelMat), 0, 0.55, -CD / 2);
  track0(back, 'wall');
  rail(CW, 0.04, 0, -CD / 2, 1.1);
  if (!hasWestNeighbour) { pane(0.02, CD, -CW / 2, 0); rail(0.04, CD, -CW / 2, 0); }
  if (!hasEastNeighbour) { pane(0.02, CD, CW / 2, 0); rail(0.04, CD, CW / 2, 0); }
  const dx = c.doorX - c.cx;
  const gapL = dx - 0.2;
  const gapR = dx + 0.2;
  const zf = CD / 2;
  pane(gapL + CW / 2, 0.02, (gapL - CW / 2) / 2, zf); rail(gapL + CW / 2, 0.04, (gapL - CW / 2) / 2, zf);
  if (CW / 2 - gapR > 0.02) { pane(CW / 2 - gapR, 0.02, (gapR + CW / 2) / 2, zf); rail(CW / 2 - gapR, 0.04, (gapR + CW / 2) / 2, zf); }
  for (const x of [gapL, gapR]) add(box(0.05, 1.0, 0.05, 0x8a5a36), x, 0.5, zf);
  add(box(0.45, 0.05, 0.05, 0x8a5a36), dx, 1.0, zf);
  const sg = sign(s.project, tint, 0.55);
  add(sg, dx - 0.2, 1.12, zf + 0.04, root);
  root.add(shell);
  const det = cabinDetails(s.project, s.seed);
  root.add(det);
  const plant = furniture('pottedPlant', { scale: 1.1 });
  plant.position.set(-0.75, 0, 0.75);
  root.add(plant);
  b.g.add(root);
  root.position.set(c.cx, 0, c.cz);
  b.track(shell, 'cabin', s.id, { snap: 'e5' });
  b.track(plant, 'plant', 'cabin-plant', { snap: 'none', onFloor: true });
  b.track(det.children[1], 'shelf', 'cabin-shelf', { snap: 'none', onFloor: true });
  b.swayList.push({ obj: plant, phase: s.seed });
}

export function buildZones(L, sessions) {
  const b = makeBuilder();
  const { g, ticks, put } = b;
  const dyn = { balls: [], prints: [] };

  for (const w of L.wings) {
    const f = flat(w.wx1 - w.wx0 + 0.1, w.z1 - w.z0 + 0.1, ACCOUNT_TINT[w.account], 0.3);
    g.add(f);
    f.position.set((w.wx0 + w.wx1) / 2, 0.006, (w.z0 + w.z1) / 2);
    w.list.forEach((s, i) => cabin(s, i % w.cols > 0, i % w.cols < w.cols - 1 && i + 1 < w.list.length, b));
  }
  for (const sp of L.spaces) spaceArea(L, sp, b, dyn, sessions);
  const takenMeet = new Set(sessions.flatMap((x) => x.subs).filter((x) => x.space === 'meeting').map((x) => x.seatIndex));
  L.spareMeeting = [0, 1, 2, 3].filter((i) => !takenMeet.has(i));
  lobby(L, b);
  const annex = buildAnnex(L, b, dyn);
  for (const f of annex.ticks) ticks.push(f);
  ticks.push((t) => { for (const s of b.swayList) s.obj.rotation.z = Math.sin(t * 0.8 + s.phase) * 0.03; });
  return { group: g, ticks, dyn, printer: dyn.printer };
}

// an unoccupied desk: same furniture as an agent's seat, monitor dark, so empty seats look intentional
export function spareDesk(b, x, z, y0 = 0, tint = 0x9aa7b4) {
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
  ret.position.set(0.565, 0, -0.2);
  const mon = new THREE.Group();
  mon.position.set(0.565, dh, -0.25);
  mon.rotation.y = -0.96;
  mon.scale.setScalar(1.05);
  mon.add(furniture('computerScreen'));
  const kb = furniture('computerKeyboard');
  kb.position.set(0, dh, -0.06);
  kb.rotation.y = Math.PI;
  const lamp = furniture('lampSquareTable', { scale: 1.1 });
  lamp.position.set(0.28, dh, 0.12);
  g.add(desk, stool, ret, mon, kb, lamp);
  b.put(g, x, y0, z, 0, null);
  b.track(desk, 'desk', 'spare', { snap: 'c25', onFloor: true, y0 });
  b.track(stool, 'chair', 'spare', { snap: 'none', onFloor: true, y0 });
  b.track(ret, 'return', 'spare', { snap: 'none', onFloor: true, y0 });
}

function spaceArea(L, sp, b, dyn, sessions) {
  const { put, g } = b;
  const w = sp.wx1 - sp.wx0;
  g.add(flat(w, sp.wz1 - sp.wz0, SPACE_TINT[sp.id], 0.26));
  g.children[g.children.length - 1].position.set(sp.cx, 0.007, (sp.wz0 + sp.wz1) / 2);
  const back = sp.wz0 + 0.25;
  const x0 = sp.wx0;
  // cable trays in front of each desk row
  const trayW = w - 1.0;
  for (let r = 0; r < 3; r++) {
    const tr = new THREE.Group();
    const t = box(trayW, 0.012, 0.07, 0x7d8590);
    const st = new THREE.Mesh(new THREE.PlaneGeometry(trayW, 0.012), new THREE.MeshBasicMaterial({ color: 0xf2b53c, toneMapped: false }));
    st.rotation.x = -Math.PI / 2; st.position.y = 0.007;
    tr.add(t, st);
    put(tr, x0 + 0.5 + trayW / 2, 0.006, sp.wz0 + 1.0 + r * ROW + 0.35, 0, null);
  }
  const sway = (o, ph) => b.swayList.push({ obj: o, phase: ph });
  if (sp.id === 'research') {
    put(furniture('cabinetTelevision', { scale: 1.0 }), x0 + 0.5, 0, back, Math.PI, 'shelf', 'x25', { onFloor: true });
    const bs = bookStack();
    put(bs, x0 + 0.5, 0.31, back, 0, null);
    const pin = board(0.6, 0.5, 0.3, pinFace(), true);
    put(pin, x0 + 1.5, 0, back, Math.PI, 'easel', 'c25', { onFloor: true });
    const gs = liveGraphScreen();
    b.ticks.push((t) => gs.userData.tick(t));
    put(board(0.9, 0.5625, 0.3, gs, true), x0 + 2.75, 0, back, Math.PI, 'easel', 'c25', { onFloor: true });
    const tbl = furniture('sideTable');
    put(tbl, x0 + 4.0, 0, back, Math.PI, 'table', 'c25', { onFloor: true });
    const pr = printer();
    put(pr, x0 + 4.0, 0.38, back, Math.PI, null);
    dyn.printer = pr;
    b.ticks.push((t) => pr.userData.tick(t));
    const mg = magnifier();
    put(mg, x0 + 3.85, 0.38, back + 0.0, 0, null);
  } else {
    const p1 = put(furniture('pottedPlant', { scale: 0.8 }), x0 + 0.25, 0, back, 0, 'plant', 'c25', { onFloor: true });
    const p2 = put(furniture('pottedPlant', { scale: 0.8 }), sp.wx1 - 0.25, 0, back, 0, 'plant', 'c25', { onFloor: true });
    sway(p1, 1); sway(p2, 3);
    put(furniture('cabinetTelevision', { scale: 1.0 }), sp.cx, 0, back, Math.PI, 'shelf', 'x25', { onFloor: true });
    put(furniture('lampRoundFloor', { scale: 1.1 }), sp.wx1 - 0.25, 0, sp.wz1 - 0.25, 0, 'lamp', 'c25', { onFloor: true });
    put(furniture('plantSmall1', { scale: 1.8 }), x0 + 0.25, 0, sp.wz1 - 0.25, 0, 'plant', 'c25', { onFloor: true });
  }
  // spare desks for every seat nobody uses yet
  const taken = new Set(sessions.flatMap((x) => x.subs).filter((x) => x.space === sp.id).map((x) => x.seatIndex));
  for (let idx = 0; idx < sp.cols * 3; idx++) {
    if (taken.has(idx)) continue;
    spareDesk(b, x0 + 1.5 + Math.floor(idx / 3) * 2.0, sp.wz0 + 1.0 + (idx % 3) * ROW, 0);
  }
  // frosted dividers between the rows, one per column, parallel to the desks
  for (let col = 0; col < sp.cols; col++) {
    for (let r = 0; r < 2; r++) {
      const dv = new THREE.Mesh(new THREE.BoxGeometry(1.4, 0.4, 0.02), std(0xcfe3ee, { transparent: true, opacity: 0.7 }));
      dv.geometry.translate(0, 0.2, 0);
      put(dv, x0 + 1.5 + col * 2.0 + 0.1, 0, sp.wz0 + 1.0 + r * ROW + 0.75, 0, 'divider', 'none', { onFloor: true });
    }
  }
}

function lobby(L, b) {
  const { put, g } = b;
  const lb = L.lobby;
  g.add(flat(lb.wx1 - lb.wx0, lb.wz1 - lb.wz0, SPACE_TINT.lobby, 0.22));
  g.children[g.children.length - 1].position.set((lb.wx0 + lb.wx1) / 2, 0.007, (lb.wz0 + lb.wz1) / 2);
  const X = (v) => L.X(v);
  const Z = (v) => L.Z(v);
  put(receptionDesk(), X(1.5), 0, Z(7.0), -Math.PI / 2, 'desk', 'c5', { onFloor: true, name: 'reception' });
  put(waterCooler(), X(1.5), 0, Z(3.5), 0, 'cooler', 'c5', { onFloor: true });
  put(umbrellaStand(), X(1.0), 0, Z(9.0), 0, 'prop', 'c5', { onFloor: true });
  put(furniture('rugDoormat', { scale: 1.2, noCast: true }), X(0.5), 0.012, L.door.z, Math.PI / 2, 'rug', 'c25');
  const pl = put(furniture('pottedPlant', { scale: 1.4 }), X(1.75), 0, Z(9.75), 0, 'plant', 'c25', { onFloor: true });
  b.swayList.push({ obj: pl, phase: 2 });
  put(furniture('plantSmall2', { scale: 1.6 }), X(1.75), 0, Z(3.25), 0, 'plant', 'c25', { onFloor: true });
  // wall furniture on the left wall: real-time clock, notice board
  const xw = L.xmin + 0.07;
  const clock = wallClock();
  put(clock, xw, 0.95, Z(5.5), Math.PI / 2, 'hung', 'none');
  b.ticks.push(() => clock.userData.tick(Date.now() / 1000));
  put(noticeBoard(3), xw, 0.75, Z(6.5), Math.PI / 2, 'hung', 'none');
  const ls = sign('LOBBY', SPACE_TINT.lobby, 0.64);
  const lp = new THREE.Mesh(new THREE.PlaneGeometry(0.64, 0.16), new THREE.MeshBasicMaterial({ map: ls.material.map, transparent: true, toneMapped: false }));
  put(lp, xw + 0.01, 1.05, Z(7.5), Math.PI / 2, 'hung', 'none');
}
