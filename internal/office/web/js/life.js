import * as THREE from 'three';

const std = (color, extra = {}) => new THREE.MeshStandardMaterial({ color, roughness: 0.85, ...extra });
const box = (w, h, d, color) => { const m = new THREE.Mesh(new THREE.BoxGeometry(w, h, d), std(color)); m.castShadow = true; return m; };

function makeCat() {
  const g = new THREE.Group();
  const fur = 0xd98a3d;
  const body = box(0.17, 0.07, 0.07, fur); body.position.set(0, 0.075, 0);
  const head = box(0.06, 0.06, 0.06, fur); head.position.set(0.11, 0.1, 0);
  const snout = box(0.02, 0.02, 0.04, 0xf4e3cf); snout.position.set(0.145, 0.09, 0);
  for (const z of [-0.022, 0.022]) { const ear = new THREE.Mesh(new THREE.ConeGeometry(0.016, 0.035, 4), std(fur)); ear.position.set(0.115, 0.145, z); g.add(ear); }
  const tail = box(0.1, 0.018, 0.018, fur); tail.geometry.translate(-0.05, 0, 0); tail.position.set(-0.085, 0.1, 0);
  g.add(body, head, snout, tail);
  const legs = [];
  for (const [x, z] of [[0.06, -0.025], [0.06, 0.025], [-0.06, -0.025], [-0.06, 0.025]]) {
    const l = box(0.018, 0.05, 0.018, fur); l.geometry.translate(0, -0.025, 0); l.position.set(x, 0.05, z); g.add(l); legs.push(l);
  }
  g.userData = { tail, legs };
  return g;
}

function makeVacuum() {
  const g = new THREE.Group();
  const disc = new THREE.Mesh(new THREE.CylinderGeometry(0.13, 0.13, 0.045, 24), std(0x30343a));
  disc.position.y = 0.025;
  disc.castShadow = true;
  const ring = new THREE.Mesh(new THREE.TorusGeometry(0.1, 0.006, 6, 24), new THREE.MeshBasicMaterial({ color: 0x7dffb0, toneMapped: false }));
  ring.rotation.x = Math.PI / 2;
  ring.position.y = 0.05;
  const dot = new THREE.Mesh(new THREE.CylinderGeometry(0.02, 0.02, 0.01, 10), std(0xcfd6dc));
  dot.position.y = 0.052;
  g.add(disc, ring, dot);
  return g;
}

function zSprite() {
  const cv = document.createElement('canvas');
  cv.width = cv.height = 64;
  const c = cv.getContext('2d');
  c.fillStyle = '#ffffff'; c.font = 'bold 44px system-ui'; c.fillText('z', 18, 46);
  const t = new THREE.CanvasTexture(cv);
  const sp = new THREE.Sprite(new THREE.SpriteMaterial({ map: t, transparent: true, depthWrite: false, toneMapped: false }));
  sp.scale.setScalar(0.12);
  sp.visible = false;
  return sp;
}

// the largest open rectangle of ground floor outside every room and clear of furniture, in cells, or null
export function openRect(L) {
  const g = L.grids[0];
  const free = (x, z) => g.open[z * g.w + x] === 1 && !L.layout.rooms.some((r) => (r.level || 0) === 0 && x >= r.x && x < r.x + r.w && z >= r.z && z < r.z + r.d);
  const heights = new Array(g.w).fill(0);
  let best = null;
  for (let z = 0; z < g.d; z++) {
    for (let x = 0; x < g.w; x++) heights[x] = free(x, z) ? heights[x] + 1 : 0;
    for (let x = 0; x < g.w; x++) {
      let h = Infinity;
      for (let x2 = x; x2 < g.w && heights[x2] > 0; x2++) {
        h = Math.min(h, heights[x2]);
        const area = (x2 - x + 1) * h;
        if (!best || area > best.area) best = { area, x, z: z - h + 1, w: x2 - x + 1, d: h };
      }
    }
  }
  return best && best.area >= 4 ? best : null;
}

// a cat that wanders the open floor and naps on a bean bag now and then, and a robot vacuum that drives a loop
// inside the largest empty rectangle. both stay out of the way when the layout has no empty floor.
export function buildLife(L) {
  const group = new THREE.Group();
  const open = openRect(L);
  const tick = () => {};
  if (!open) return { group, tick };
  const cat = makeCat();
  group.add(cat);
  const zs = zSprite();
  group.add(zs);
  const X0 = L.X(open.x) + 0.25;
  const X1 = L.X(open.x + open.w) - 0.25;
  const Z0 = L.Z(open.z) + 0.25;
  const Z1 = L.Z(open.z + open.d) - 0.25;
  const catZ = (Z0 + Z1) / 2;
  const bagSpot = (L.groups.bag || [])[1] || (L.groups.bag || [])[0];
  const bagTop = 0.3;
  // the way to the bean bag and back, found over the walkable grid
  let toBag = null;
  if (bagSpot) {
    const home = L.node(0, { x: X0 + 0.5, z: catZ });
    toBag = L.path(home, bagSpot.node).map((p) => ({ x: p.x, z: p.z, y: 0 }));
    toBag.push({ x: bagSpot.x, z: bagSpot.z, y: bagTop });
  }
  const c = { x: X0 + 0.5, z: catZ, y: 0, tx: X0 + 1.5, pause: 0, mode: 'wander', next: 40, wp: 0, sleepEnd: 0 };
  cat.position.set(c.x, 0, c.z);
  const vac = makeVacuum();
  group.add(vac);
  const loop = [[X0, Z0], [Math.max(X0 + 0.5, X1), Z0], [Math.max(X0 + 0.5, X1), Math.max(Z0 + 0.5, Z1)], [X0, Math.max(Z0 + 0.5, Z1)]];
  const legLen = loop.map((p, i) => Math.hypot(loop[(i + 1) % 4][0] - p[0], loop[(i + 1) % 4][1] - p[1]));
  const total = legLen.reduce((a, b) => a + b, 0);
  const place = (dist) => {
    let d = dist % total;
    for (let i = 0; i < 4; i++) {
      if (d <= legLen[i]) {
        const p = loop[i];
        const q = loop[(i + 1) % 4];
        const u = d / legLen[i];
        return { x: p[0] + (q[0] - p[0]) * u, z: p[1] + (q[1] - p[1]) * u, yaw: Math.atan2(q[0] - p[0], q[1] - p[1]) };
      }
      d -= legLen[i];
    }
    return { x: loop[0][0], z: loop[0][1], yaw: 0 };
  };
  const walkTo = (tx, tz, ty, speed, dt) => {
    const dx = tx - c.x;
    const dz = tz - c.z;
    const dist = Math.hypot(dx, dz);
    const step = speed * dt;
    if (dist <= step) { c.x = tx; c.z = tz; c.y = ty; return true; }
    c.x += (dx / dist) * step; c.z += (dz / dist) * step;
    c.y += (ty - c.y) * Math.min(1, step / Math.max(dist, 0.05));
    return false;
  };
  const tickLife = (dt, t) => {
    let walking = false;
    if (c.mode === 'wander') {
      if (c.pause > 0) c.pause -= dt;
      else {
        walking = true;
        if (walkTo(c.tx, catZ, 0, 0.25, dt)) { walking = false; c.pause = 3; c.tx = X0 + ((Math.sin(t * 1.7) * 0.5 + 0.5) * Math.max(0.5, X1 - X0)); }
        cat.rotation.y = c.tx > c.x ? 0 : Math.PI;
      }
      if (toBag && t > c.next) { c.mode = 'toBag'; c.wp = 0; }
    } else if (c.mode === 'toBag' || c.mode === 'back') {
      walking = true;
      const out = c.mode === 'toBag';
      const w = toBag[c.wp];
      if (walkTo(w.x, w.z, w.y, 0.25, dt)) {
        c.wp += out ? 1 : -1;
        if (out && c.wp >= toBag.length) { c.mode = 'sleep'; c.sleepEnd = t + 26; c.wp = toBag.length - 1; }
        if (!out && c.wp < 0) { c.mode = 'wander'; c.next = t + 55; c.y = 0; }
      }
      const nw = toBag[Math.max(0, Math.min(toBag.length - 1, c.wp))];
      cat.rotation.y = -Math.atan2(nw.z - c.z, nw.x - c.x);
    } else if (t > c.sleepEnd) c.mode = 'back';
    cat.position.set(c.x, c.y, c.z);
    const sleeping = c.mode === 'sleep';
    cat.userData.legs.forEach((l, i) => { l.scale.y = sleeping ? 0.25 : 1; l.rotation.z = walking ? Math.sin(t * 9 + (i % 2) * Math.PI) * 0.5 : 0; });
    cat.userData.tail.rotation.y = sleeping ? 1.1 : Math.sin(t * 2.2) * 0.5;
    cat.scale.y = sleeping ? 0.8 : 1;
    zs.visible = sleeping;
    if (sleeping) { const p = (t * 0.4) % 1; zs.position.set(c.x, c.y + 0.2 + p * 0.15, c.z); zs.material.opacity = Math.sin(p * Math.PI); }
    const v = place(t * 0.14);
    vac.position.set(v.x, 0, v.z);
    vac.rotation.y = v.yaw;
  };
  return { group, tick: tickLife };
}
