// canvas textures of the garden, cached per build. the page's csp allows canvas drawing, no images are fetched.
import * as THREE from 'three';
import { rng } from './geo.js';
import { cached, track } from './res.js';

const hex = (n) => '#' + n.toString(16).padStart(6, '0');
const hsl = (h, s, l) => `hsl(${h},${s}%,${l}%)`;

export function canvasTex(w, h, draw) {
  const cv = document.createElement('canvas');
  cv.width = w; cv.height = h;
  draw(cv.getContext('2d'), w, h);
  const t = new THREE.CanvasTexture(cv);
  t.colorSpace = THREE.SRGBColorSpace;
  t.anisotropy = 4;
  return track(t);
}

function tiled(key, w, h, draw, rx = 1, ry = 1) {
  return cached(key, () => {
    const t = canvasTex(w, h, draw);
    t.wrapS = t.wrapT = THREE.RepeatWrapping;
    t.repeat.set(rx, ry);
    return t;
  });
}

// wood planks running along x, one tile is 1.5 world units square
export function plankTex(w, d, hue = 30, light = 52) {
  const r = rng(7);
  return tiled(`t:plank:${w}:${d}:${hue}:${light}`, 256, 256, (c, W, H) => {
    c.fillStyle = hsl(hue, 30, light - 22); c.fillRect(0, 0, W, H);
    const n = 6;
    for (let i = 0; i < n; i++) {
      const y = (i * H) / n;
      c.fillStyle = hsl(hue + r() * 6 - 3, 38 + r() * 8, light + r() * 8 - 4);
      c.fillRect(0, y + 2, W, H / n - 4);
      c.fillStyle = 'rgba(80,45,20,0.12)';
      for (let k = 0; k < 4; k++) c.fillRect(r() * W, y + 4 + r() * (H / n - 10), 30 + r() * 60, 1.5);
      c.fillStyle = hsl(hue, 30, light - 24);
      c.fillRect((r() * 0.8 + 0.1) * W, y + 2, 2, H / n - 4);
    }
  }, w / 1.5, d / 1.5);
}

// pale flagstones for plazas and paths
export function flagTex(w, d, hue = 38) {
  const r = rng(11);
  return tiled(`t:flag:${w}:${d}:${hue}`, 256, 256, (c, W, H) => {
    c.fillStyle = hsl(hue, 14, 62); c.fillRect(0, 0, W, H);
    const n = 4;
    for (let i = 0; i < n; i++) for (let j = 0; j < n; j++) {
      c.fillStyle = hsl(hue + r() * 8 - 4, 14 + r() * 6, 76 + r() * 10 - 5);
      c.fillRect(i * (W / n) + 3, j * (H / n) + 3, W / n - 6, H / n - 6);
    }
  }, w / 2, d / 2);
}

// lawn with soft darker and lighter patches
export function grassTex(rep = 0.25) {
  const r = rng(3);
  return tiled(`t:grass:${rep}`, 256, 256, (c, W, H) => {
    c.fillStyle = hsl(100, 42, 52); c.fillRect(0, 0, W, H);
    for (let i = 0; i < 90; i++) {
      c.fillStyle = hsl(95 + r() * 20, 40 + r() * 10, 44 + r() * 14);
      c.globalAlpha = 0.28;
      c.beginPath(); c.arc(r() * W, r() * H, 8 + r() * 26, 0, 7); c.fill();
    }
    c.globalAlpha = 0.55;
    for (let i = 0; i < 260; i++) {
      c.fillStyle = hsl(90 + r() * 25, 45, 36 + r() * 24);
      c.fillRect(r() * W, r() * H, 1.5, 3 + r() * 3);
    }
  }, rep, rep);
}

// streaks that scroll slowly across water
export function waterTex() {
  const r = rng(4);
  return tiled('t:water', 256, 256, (c) => {
    c.fillStyle = '#ffffff';
    c.fillRect(0, 0, 256, 256);
    c.strokeStyle = 'rgba(150,215,240,0.9)';
    c.lineWidth = 3;
    c.lineCap = 'round';
    for (let i = 0; i < 34; i++) {
      const x = r() * 256;
      const y = r() * 256;
      c.beginPath();
      c.moveTo(x, y);
      c.quadraticCurveTo(x + 14, y - 5, x + 30 + r() * 20, y);
      c.stroke();
    }
  }, 0.18, 0.18);
}

// striped fabric top for awnings and umbrellas
export function stripeTex(c1, n = 8, rep = 1) {
  return cached(`t:stripe:${c1}:${n}:${rep}`, () => {
    const t = canvasTex(128, 32, (c) => {
      for (let i = 0; i < n; i++) { c.fillStyle = i % 2 ? '#fbf6ea' : hex(c1); c.fillRect(i * (128 / n), 0, 128 / n, 32); }
    });
    t.wrapS = THREE.RepeatWrapping;
    t.repeat.set(rep, 1);
    return t;
  });
}

export function checkTex() {
  return cached('t:picnic', () => canvasTex(128, 128, (c) => {
    for (let i = 0; i < 8; i++) for (let j = 0; j < 8; j++) { c.fillStyle = (i + j) % 2 ? '#fbf3e4' : '#e8647c'; c.fillRect(i * 16, j * 16, 16, 16); }
  }));
}

// dark rounded tag with an accent outline, used on cabin signs and the lobby arch
export function textSprite(text, accent, scale = 0.64) {
  const t = cached(`t:label:${text}:${accent}`, () => canvasTex(512, 128, (c) => {
    c.fillStyle = 'rgba(24,18,28,0.88)';
    c.beginPath(); c.roundRect(6, 14, 500, 100, 28); c.fill();
    c.lineWidth = 8; c.strokeStyle = hex(accent); c.stroke();
    c.fillStyle = '#fff'; c.font = '600 54px system-ui, sans-serif';
    c.textAlign = 'center'; c.textBaseline = 'middle';
    c.fillText(text, 256, 66);
  }));
  const sp = new THREE.Sprite(cached(`m:label:${text}:${accent}`, () => new THREE.SpriteMaterial({ map: t, transparent: true, depthWrite: false, toneMapped: false, fog: false })));
  sp.scale.set(scale, scale / 4, 1);
  sp.renderOrder = 10;
  return sp;
}

export function whiteboardTex(seed) {
  return cached(`t:board:${seed % 3}`, () => canvasTex(256, 160, (c, w, h) => {
    c.fillStyle = '#f7f6f1'; c.fillRect(0, 0, w, h);
    c.strokeStyle = ['#2b5fa8', '#c0392b', '#2f7d4f'][seed % 3]; c.lineWidth = 3; c.lineCap = 'round';
    c.beginPath(); c.moveTo(14, 30); c.lineTo(90, 30); c.moveTo(14, 55); c.lineTo(130, 55); c.moveTo(14, 80); c.lineTo(70, 80); c.stroke();
    c.beginPath(); c.arc(190, 60, 28, 0, 7); c.moveTo(190, 88); c.lineTo(190, 130); c.stroke();
  }));
}
