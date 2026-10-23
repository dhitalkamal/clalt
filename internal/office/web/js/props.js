import * as THREE from 'three';

const hex = (n) => '#' + n.toString(16).padStart(6, '0');
const std = (color, extra = {}) => new THREE.MeshStandardMaterial({ color, roughness: 0.7, ...extra });

// seat without backrest or armrests so the character is never hidden or clipped. three variants for variety.
export function makeStool(tint, variant = 0) {
  const g = new THREE.Group();
  const seatGeo = variant === 1 ? new THREE.BoxGeometry(0.28, 0.05, 0.28) : new THREE.CylinderGeometry(0.15, 0.15, 0.05, 20);
  const seat = new THREE.Mesh(seatGeo, std(tint));
  seat.position.y = 0.375;
  const post = new THREE.Mesh(new THREE.CylinderGeometry(0.02, 0.02, 0.35, 8), std(0x8a9696, { metalness: 0.4 }));
  post.position.y = 0.19;
  const base = new THREE.Mesh(variant === 2 ? new THREE.CylinderGeometry(0.14, 0.14, 0.02, 24) : new THREE.CylinderGeometry(0.13, 0.13, 0.02, 5), std(0x6b7777));
  base.position.y = 0.01;
  g.add(seat, post, base);
  if (variant === 2) { const ring = new THREE.Mesh(new THREE.TorusGeometry(0.1, 0.008, 6, 20), std(0x8a9696, { metalness: 0.4 })); ring.rotation.x = Math.PI / 2; ring.position.y = 0.18; g.add(ring); }
  g.traverse((o) => { if (o.isMesh) { o.castShadow = true; o.receiveShadow = true; } });
  return g;
}

export function makeCrown() {
  const g = new THREE.Group();
  const mat = std(0xf2c14b, { metalness: 0.6, roughness: 0.3, emissive: 0x6b4a00, emissiveIntensity: 0.5 });
  const band = new THREE.Mesh(new THREE.CylinderGeometry(0.05, 0.045, 0.03, 12, 1, true), mat);
  mat.side = THREE.DoubleSide;
  g.add(band);
  for (let i = 0; i < 5; i++) {
    const a = (i / 5) * Math.PI * 2;
    const tip = new THREE.Mesh(new THREE.ConeGeometry(0.014, 0.04, 6), mat);
    tip.position.set(Math.cos(a) * 0.047, 0.03, Math.sin(a) * 0.047);
    g.add(tip);
  }
  return g;
}

// six dots circling above the head, color of the session account
export function makeRing(color) {
  const g = new THREE.Group();
  const geo = new THREE.SphereGeometry(0.014, 8, 6);
  const mat = new THREE.MeshBasicMaterial({ color, toneMapped: false });
  for (let i = 0; i < 6; i++) {
    const d = new THREE.Mesh(geo, mat);
    const a = (i / 6) * Math.PI * 2;
    d.position.set(Math.cos(a) * 0.08, 0, Math.sin(a) * 0.08);
    d.userData.i = i;
    g.add(d);
  }
  return g;
}

export function textSprite(text, accent, sub) {
  const cv = document.createElement('canvas');
  cv.width = 512; cv.height = 128;
  const c = cv.getContext('2d');
  c.fillStyle = 'rgba(24,18,28,0.88)';
  c.beginPath(); c.roundRect(6, 14, 500, 100, 28); c.fill();
  c.lineWidth = 8; c.strokeStyle = hex(accent); c.stroke();
  c.fillStyle = '#fff'; c.font = '600 54px system-ui, sans-serif';
  c.textAlign = 'center'; c.textBaseline = 'middle';
  c.fillText(text, 256, sub ? 52 : 66);
  if (sub) { c.fillStyle = hex(accent); c.font = '36px system-ui, sans-serif'; c.fillText(sub, 256, 92); }
  const tex = new THREE.CanvasTexture(cv);
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.anisotropy = 4;
  const sp = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, transparent: true, depthWrite: false, toneMapped: false, fog: false }));
  sp.scale.set(0.64, 0.16, 1);
  sp.renderOrder = 10;
  return sp;
}

let glowTex = null;
export function glowTexture() {
  if (glowTex) return glowTex;
  const cv = document.createElement('canvas');
  cv.width = cv.height = 128;
  const c = cv.getContext('2d');
  const g = c.createRadialGradient(64, 64, 0, 64, 64, 64);
  g.addColorStop(0, 'rgba(255,255,255,1)');
  g.addColorStop(0.4, 'rgba(255,255,255,0.35)');
  g.addColorStop(1, 'rgba(255,255,255,0)');
  c.fillStyle = g; c.fillRect(0, 0, 128, 128);
  glowTex = new THREE.CanvasTexture(cv);
  glowTex.userData.shared = true;
  return glowTex;
}

export function glowSprite(color, size, opacity) {
  const sp = new THREE.Sprite(new THREE.SpriteMaterial({
    map: glowTexture(), color, transparent: true, opacity, depthWrite: false,
    blending: THREE.AdditiveBlending, toneMapped: false,
  }));
  sp.scale.setScalar(size);
  return sp;
}

export function makeMug(color) {
  const m = new THREE.Mesh(new THREE.CylinderGeometry(0.018, 0.016, 0.04, 10), std(color));
  m.castShadow = true;
  return m;
}

export function makePaper(rot) {
  const m = new THREE.Mesh(new THREE.BoxGeometry(0.09, 0.006, 0.12), std(0xf2eee4));
  m.rotation.y = rot;
  m.castShadow = true;
  return m;
}
