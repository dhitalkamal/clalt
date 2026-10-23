import * as THREE from 'three';

const LINES = [
  'read src/auth/session.ts', 'edit  src/auth/session.ts (+14 -3)', 'bash  npm test -- --watch=false',
  'PASS  auth/session.test.ts', 'grep  "refreshToken" -r src', 'read  package.json', 'think: plan the migration',
  'edit  api/routes/users.ts', 'bash  git diff --stat', 'write docs/notes.md', 'bash  go vet ./...',
  'tool  search_files', 'PASS  api/users.test.ts', 'edit  README.md (+6)', 'bash  cargo check',
];

// small canvas texture used as the monitor surface, redrawn from the session state
export class ScreenTex {
  constructor(seed) {
    this.canvas = document.createElement('canvas');
    this.canvas.width = 640;
    this.canvas.height = 400;
    this.ctx = this.canvas.getContext('2d');
    this.ctx.scale(2, 2);
    this.tex = new THREE.CanvasTexture(this.canvas);
    this.tex.colorSpace = THREE.SRGBColorSpace;
    this.tex.anisotropy = 4;
    this.seed = seed;
    this.last = -1;
  }

  draw(state, t) {
    const step = Math.floor(t * 12);
    if (step === this.last && state !== 'needs-you') return;
    this.last = step;
    const c = this.ctx;
    c.fillStyle = state === 'ended' ? '#050505' : '#0a1220';
    c.fillRect(0, 0, 320, 200);
    if (state === 'ended') { this.tex.needsUpdate = true; return; }
    c.font = 'bold 13px ui-monospace, Menlo, monospace';
    const off = (t * 26) % 18;
    const first = Math.floor(t * 26 / 18);
    const dim = state === 'idle' ? 0.35 : 1;
    c.globalAlpha = dim;
    for (let i = 0; i < 13; i++) {
      const line = LINES[(((first + i + this.seed * 3) % LINES.length) + LINES.length) % LINES.length];
      const age = i;
      c.fillStyle = line.startsWith('PASS') ? '#7dffb0' : age % 5 === 0 ? '#ffd27a' : '#d6e8ff';
      c.fillText((line.startsWith('PASS') ? '' : '> ') + line, 8, 200 - i * 18 + off - 4);
    }
    c.globalAlpha = 1;
    c.fillStyle = '#0a1220';
    c.fillRect(0, 0, 320, 22);
    c.fillStyle = state === 'busy' ? '#7dffb0' : state === 'idle' ? '#7a8594' : '#f2b53c';
    c.fillText(state === 'busy' ? 'working...' : state === 'idle' ? 'idle - waiting' : 'needs you', 8, 15);
    if (state === 'needs-you') {
      const pulse = 0.55 + 0.45 * Math.sin(t * 5);
      c.fillStyle = `rgba(242,181,60,${0.2 + 0.25 * pulse})`;
      c.fillRect(30, 70, 260, 60);
      c.strokeStyle = '#f2b53c';
      c.strokeRect(30, 70, 260, 60);
      c.fillStyle = '#fff3d6';
      c.font = 'bold 16px ui-monospace, Menlo, monospace';
      c.fillText('Allow Bash: rm -rf dist ?', 46, 96);
      c.font = 'bold 13px ui-monospace, Menlo, monospace';
      c.fillText('[y] yes   [n] no', 46, 118);
    }
    if (state === 'idle' && Math.floor(t * 2) % 2 === 0) { c.fillStyle = '#d6e8ff'; c.fillRect(10, 186, 8, 3); }
    this.tex.needsUpdate = true;
  }
}

// "!" bubble with a soft glow, drawn once
export function bubbleTexture() {
  const cv = document.createElement('canvas');
  cv.width = cv.height = 256;
  const c = cv.getContext('2d');
  const grd = c.createRadialGradient(128, 128, 20, 128, 128, 126);
  grd.addColorStop(0, 'rgba(255,190,60,0.85)');
  grd.addColorStop(0.45, 'rgba(255,170,40,0.28)');
  grd.addColorStop(1, 'rgba(255,170,40,0)');
  c.fillStyle = grd;
  c.fillRect(0, 0, 256, 256);
  c.beginPath();
  c.arc(128, 128, 62, 0, Math.PI * 2);
  c.fillStyle = '#ffb938';
  c.fill();
  c.lineWidth = 6;
  c.strokeStyle = '#fff1c9';
  c.stroke();
  c.fillStyle = '#3b2400';
  c.font = 'bold 96px system-ui, sans-serif';
  c.textAlign = 'center';
  c.textBaseline = 'middle';
  c.fillText('!', 128, 134);
  const tex = new THREE.CanvasTexture(cv);
  tex.colorSpace = THREE.SRGBColorSpace;
  return tex;
}
