import * as THREE from 'three';

// speech bubble pictures for what an agent is working on, drawn once each and shared
const cache = {};

function bubble(draw, bg = '#ffffff') {
  const cv = document.createElement('canvas');
  cv.width = cv.height = 128;
  const g = cv.getContext('2d');
  g.fillStyle = bg;
  g.beginPath(); g.roundRect(8, 8, 112, 92, 28); g.fill();
  g.beginPath(); g.moveTo(52, 98); g.lineTo(64, 120); g.lineTo(76, 98); g.fill();
  g.lineWidth = 8; g.lineCap = 'round'; g.lineJoin = 'round';
  draw(g);
  const t = new THREE.CanvasTexture(cv);
  t.colorSpace = THREE.SRGBColorSpace;
  t.userData.shared = true;
  return t;
}

const white = (g) => { g.strokeStyle = '#fff'; g.lineWidth = 12; };

const DRAW = {
  read: [(g) => {
    g.strokeStyle = '#3f6fa3';
    g.beginPath(); g.moveTo(30, 34); g.lineTo(64, 42); g.lineTo(98, 34); g.lineTo(98, 78); g.lineTo(64, 86); g.lineTo(30, 78); g.closePath();
    g.moveTo(64, 42); g.lineTo(64, 86); g.stroke();
  }],
  type: [(g) => { g.fillStyle = '#3f7a2e'; for (let r = 0; r < 3; r++) for (let c = 0; c < 5; c++) g.fillRect(26 + c * 16, 34 + r * 16, 11, 11); }],
  wait: [(g) => {
    g.strokeStyle = '#b26b00';
    g.beginPath(); g.moveTo(40, 26); g.lineTo(88, 26); g.lineTo(64, 54); g.lineTo(88, 82); g.lineTo(40, 82); g.lineTo(64, 54); g.closePath(); g.stroke();
  }],
  web: [(g) => { g.strokeStyle = '#3f6fa3'; g.beginPath(); g.arc(58, 50, 20, 0, 7); g.moveTo(72, 64); g.lineTo(90, 82); g.stroke(); }],
  pass: [(g) => { white(g); g.beginPath(); g.moveTo(36, 56); g.lineTo(56, 76); g.lineTo(92, 34); g.stroke(); }, '#3f9e5a'],
  fail: [(g) => { white(g); g.beginPath(); g.moveTo(40, 32); g.lineTo(88, 80); g.moveTo(88, 32); g.lineTo(40, 80); g.stroke(); }, '#c94f3a'],
};

// the texture for an icon name from behave/look.js
export function workIcon(name) {
  if (!cache[name]) cache[name] = bubble(...DRAW[name]);
  return cache[name];
}
