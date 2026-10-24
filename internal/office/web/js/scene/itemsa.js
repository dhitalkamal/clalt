// item builders for plain furniture and decor. every builder returns an object whose origin is the footprint
// centre on the floor, front toward +z at rot 0. ctx gives b (ticks, sway list, lamp list) and the layout L.
import * as THREE from 'three';
import { furniture } from '../assets.js';
import { std, box, wallClock, noticeBoard, liveGraphScreen } from '../details.js';
import { receptionDesk, umbrellaStand, waterCooler, printer, pinFace, magnifier, reviewPages, markerTray, coffeePot } from '../small.js';
import { whiteboardTexture } from '../art.js';
import { glowSprite } from '../props.js';
import { board } from '../zones.js';

const sway = (c, obj, phase = 0) => { c.b.sway.push({ obj, phase: phase + c.n }); return obj; };
const glb = (name, scale, opts = {}) => () => furniture(name, { scale, ...opts });
const plant = (name, scale) => (c) => sway(c, furniture(name, { scale }));
const lamp = (name) => (c) => { const g = furniture(name, { scale: 1.1 }); c.b.lamps.push(g); return g; };

// a side table with something on top, lifted by the table's own height
function onTable(make, dy = 0.38) {
  return (c, thing = make(c)) => {
    const g = new THREE.Group();
    g.add(furniture('sideTable'));
    thing.position.y = dy;
    g.add(thing);
    return g;
  };
}

function easel(c, w, h, face, y = 0.3) {
  const g = board(w, h, y, face, true);
  g.rotation.y = Math.PI;
  const out = new THREE.Group();
  out.add(g);
  return out;
}

export const BUILD_A = {
  plant: plant('pottedPlant', 1.3),
  'plant-small': plant('pottedPlant', 0.8),
  fern: plant('plantSmall1', 1.8),
  sprout: plant('plantSmall2', 1.6),
  'lamp-floor': lamp('lampRoundFloor'),
  'lamp-square': lamp('lampSquareFloor'),
  bookcase: (c) => { const g = furniture('bookcaseClosed', { scale: 1.2 }); g.rotation.y = Math.PI; return g; },
  'bookcase-open': (c) => { const g = furniture('bookcaseOpen', { scale: 1.2 }); g.rotation.y = Math.PI; return g; },
  sofa: glb('loungeSofa', 1.6),
  'lounge-chair': glb('loungeChair', 1.2),
  'coffee-table': glb('tableCoffee', 1.5),
  rug: (c) => lift(furniture('rugRectangle', { scale: 1.9, noCast: true })),
  'rug-large': (c) => lift(furniture('rugRectangle', { scale: 2.3, noCast: true })),
  'rug-round': (c) => lift(furniture('rugRound', { scale: 2.0, noCast: true })),
  'rug-small': (c) => lift(furniture('rugRectangle', { scale: 1.0, noCast: true })),
  'side-table': () => furniture('sideTable'),
  'round-table': () => {
    const tb = furniture('tableRound');
    const sz = tb.userData.size;
    tb.scale.set(1.0 / sz.x, 0.38 / sz.y, 1.0 / sz.z);
    return tb;
  },
  'printer-table': (c) => {
    const g = onTable(() => { const p = printer(); c.b.printers.push(p); c.b.ticks.push((t) => p.userData.tick(t)); return p; })(c);
    const mg = magnifier();
    mg.position.set(-0.15, 0.38, 0);
    g.add(mg);
    return g;
  },
  'pages-table': onTable(() => reviewPages()),
  'coffee-pot-table': onTable(() => coffeePot()),
  'tv-cabinet': () => { const g = furniture('cabinetTelevision', { scale: 1.0 }); g.rotation.y = Math.PI; return wrap(g); },
  'pin-board': (c) => easel(c, 0.6, 0.5, pinFace()),
  'graph-screen': (c) => {
    const gs = liveGraphScreen();
    c.b.ticks.push((t) => gs.userData.tick(t));
    return easel(c, 0.9, 0.5625, gs);
  },
  whiteboard: (c) => {
    const face = new THREE.Mesh(new THREE.PlaneGeometry(1, 1), new THREE.MeshBasicMaterial({ map: whiteboardTexture(), toneMapped: false }));
    const g = easel(c, 1.3, 0.7, face);
    const tray = markerTray();
    tray.position.set(0, 0.3, -0.04);
    tray.rotation.y = Math.PI;
    g.add(tray);
    return g;
  },
  'menu-board': (c) => easel(c, 0.5, 0.7, menuFace()),
  rack: () => { const m = box(0.4, 0.9, 0.3, 0x30343a); m.position.y = 0.45; return wrap(m); },
  divider: () => {
    const dv = new THREE.Mesh(new THREE.BoxGeometry(1.4, 0.4, 0.02), std(0xcfe3ee, { transparent: true, opacity: 0.7 }));
    dv.geometry.translate(0, 0.2, 0);
    return wrap(dv);
  },
  // wall items hang against the back (north at rot 0) edge of their cell
  clock: (c) => { const m = wallClock(); c.b.ticks.push(() => m.userData.tick(Date.now() / 1000)); m.position.set(0, 0.95, -0.18); return wrap(m); },
  'notice-board': () => { const m = noticeBoard(3); m.position.set(0, 0.75, -0.18); return wrap(m); },
  pendant: () => {
    const H = 1.95;
    const pg = new THREE.Group();
    const cord = new THREE.Mesh(new THREE.CylinderGeometry(0.004, 0.004, 0.3, 4), std(0x222222));
    cord.position.y = H - 0.17;
    const shade = new THREE.Mesh(new THREE.ConeGeometry(0.09, 0.08, 14, 1, true), std(0xf2d9a8, { side: THREE.DoubleSide, emissive: 0xffc27a, emissiveIntensity: 0.6 }));
    shade.position.y = H - 0.36;
    const bulb = glowSprite(0xffd9a0, 0.22, 0.7);
    bulb.position.y = H - 0.4;
    pg.add(cord, shade, bulb);
    return pg;
  },
  reception: () => receptionDesk(),
  'umbrella-stand': () => umbrellaStand(),
};

function menuFace() {
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

// rugs sit a hair above the floor so they never z-fight with the room tint
function lift(o) { o.position.y = 0.012; return wrap(o); }
function wrap(o) { const g = new THREE.Group(); g.add(o); return g; }

export { sway };
