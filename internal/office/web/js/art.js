import * as THREE from 'three';

function canvas(w, h) {
  const cv = document.createElement('canvas');
  cv.width = w; cv.height = h;
  return [cv, cv.getContext('2d')];
}
function tex(cv) {
  const t = new THREE.CanvasTexture(cv);
  t.colorSpace = THREE.SRGBColorSpace;
  t.anisotropy = 4;
  return t;
}

// sky seen through the windows: blue on top, warm near the horizon, a few soft clouds
export function skyTexture() {
  const [cv, c] = canvas(128, 256);
  const g = c.createLinearGradient(0, 0, 0, 256);
  g.addColorStop(0, '#5aa2e6'); g.addColorStop(0.55, '#a9d3f5'); g.addColorStop(0.85, '#ffe3b8'); g.addColorStop(1, '#ffc58e');
  c.fillStyle = g; c.fillRect(0, 0, 128, 256);
  c.fillStyle = 'rgba(255,255,255,0.7)';
  for (const [x, y, r] of [[30, 90, 18], [52, 96, 24], [78, 88, 16], [90, 150, 14], [70, 156, 20]]) { c.beginPath(); c.arc(x, y, r, 0, 7); c.fill(); }
  return tex(cv);
}

export function paintingTexture(v) {
  const [cv, c] = canvas(160, 120);
  const pal = [['#274c77', '#e7c6a0', '#c8553d'], ['#2f5d50', '#f1e3c6', '#d98a3d'], ['#5b3a6b', '#f2d5a0', '#4f86c6']][v % 3];
  c.fillStyle = pal[1]; c.fillRect(0, 0, 160, 120);
  c.fillStyle = pal[0]; c.beginPath(); c.moveTo(0, 120); c.lineTo(55, 50); c.lineTo(100, 100); c.lineTo(125, 70); c.lineTo(160, 120); c.fill();
  c.fillStyle = pal[2]; c.beginPath(); c.arc(115, 34, 16, 0, 7); c.fill();
  return tex(cv);
}

export function whiteboardTexture() {
  const [cv, c] = canvas(512, 280);
  c.fillStyle = '#f7f6f1'; c.fillRect(0, 0, 512, 280);
  c.lineWidth = 3; c.lineCap = 'round';
  c.strokeStyle = '#2b5fa8'; c.strokeRect(30, 30, 120, 50); c.strokeRect(200, 30, 120, 50); c.strokeRect(370, 30, 110, 50);
  c.beginPath(); c.moveTo(150, 55); c.lineTo(200, 55); c.moveTo(320, 55); c.lineTo(370, 55); c.stroke();
  c.strokeStyle = '#c0392b'; c.beginPath(); c.moveTo(40, 130); c.bezierCurveTo(120, 100, 180, 170, 260, 130); c.bezierCurveTo(330, 100, 400, 170, 480, 120); c.stroke();
  c.strokeStyle = '#2f7d4f';
  for (let i = 0; i < 4; i++) { c.beginPath(); c.moveTo(40, 190 + i * 22); c.lineTo(120 + ((i * 53) % 150), 190 + i * 22); c.stroke(); c.strokeRect(310, 185 + i * 22, 12, 12); }
  c.fillStyle = '#f2d04a'; c.fillRect(400, 175, 70, 70); c.fillStyle = '#f08fa0'; c.fillRect(350, 205, 40, 40);
  return tex(cv);
}

export function wallScreenTexture() {
  const [cv, c] = canvas(512, 320);
  c.fillStyle = '#0a1220'; c.fillRect(0, 0, 512, 320);
  const nodes = [[256, 50], [120, 130], [256, 140], [390, 130], [70, 230], [170, 235], [330, 232], [440, 230]];
  c.strokeStyle = '#4f86c6'; c.lineWidth = 3;
  [[0, 1], [0, 2], [0, 3], [1, 4], [1, 5], [3, 6], [3, 7]].forEach(([a, b]) => { c.beginPath(); c.moveTo(...nodes[a]); c.lineTo(...nodes[b]); c.stroke(); });
  nodes.forEach(([x, y], i) => { c.fillStyle = i % 3 === 0 ? '#ffd27a' : '#7dffb0'; c.beginPath(); c.arc(x, y, 14, 0, 7); c.fill(); });
  c.fillStyle = '#d6e8ff'; c.font = 'bold 22px ui-monospace, Menlo, monospace'; c.fillText('call graph', 16, 300);
  return tex(cv);
}
