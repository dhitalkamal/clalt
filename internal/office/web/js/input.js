// pointer, wheel, keys and buttons: orbit, pan, zoom, hover card and the camera buttons
import * as THREE from 'three';
import { zoomAt, resetZoom, setPreset, orbit, pan } from './stage.js';
import { app, stage, ui, canvas } from './app.js';

const ray = new THREE.Raycaster();
const ndc = new THREE.Vector2();
const floorP = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0);
const hitP = new THREE.Vector3();
let mouse = null;
let down = null;
// one pointer drags to orbit (right button or shift pans), two touches pinch-zoom and pan. a click that was a drag is not a click.
const pointers = new Map();
let pinch = 0;

function castFrom(x, y) {
  ndc.set((x / innerWidth) * 2 - 1, -(y / innerHeight) * 2 + 1);
  ray.setFromCamera(ndc, stage.camera);
}

// the smallest region under a screen point, trying the deck level first
function pickRegion(x, y) {
  castFrom(x, y);
  for (const level of [1.29, 0]) {
    floorP.constant = -level;
    if (!ray.ray.intersectPlane(floorP, hitP)) continue;
    const hits = app.layout.regions.filter((r) => (r.y || 0) === level && hitP.x >= r.x0 && hitP.x <= r.x1 && hitP.z >= r.z0 && hitP.z <= r.z1);
    hits.sort((a, c) => (a.x1 - a.x0) * (a.z1 - a.z0) - (c.x1 - c.x0) * (c.z1 - c.z0));
    if (hits[0]) return hits[0];
  }
  return null;
}

function regionCard(r) {
  const staff = [...app.agents.values()].filter((a) => a.mode !== 'away' && a.rig.position.x > r.x0 && a.rig.position.x < r.x1 && a.rig.position.z > r.z0 && a.rig.position.z < r.z1).length;
  return { title: r.label || r.type || r.id, staff };
}

const endPointer = (e) => {
  const wasDrag = down?.moved;
  pointers.delete(e.pointerId);
  if (pointers.size < 2) pinch = 0;
  if (pointers.size === 0) {
    app.dragging = false;
    if (!wasDrag && e.type === 'pointerup') {
      app.selected = app.hover || null;
      app.selectedRegion = app.hover ? null : pickRegion(e.clientX, e.clientY);
      app.selectPos = { x: e.clientX, y: e.clientY };
    }
    down = null;
  }
};

function onMove(e) {
  mouse = { x: e.clientX, y: e.clientY };
  const p = pointers.get(e.pointerId);
  if (!p) return;
  const dx = e.clientX - p.x;
  const dy = e.clientY - p.y;
  p.x = e.clientX; p.y = e.clientY;
  if (down && Math.hypot(e.clientX - down.x, e.clientY - down.y) > 5) down.moved = true;
  if (!down?.moved) return;
  app.dragging = true;
  if (pointers.size >= 2) {
    const [a, c] = [...pointers.values()];
    const d = Math.hypot(a.x - c.x, a.y - c.y);
    if (pinch) zoomAt(stage, new THREE.Vector2(((a.x + c.x) / 2 / innerWidth) * 2 - 1, -((a.y + c.y) / 2 / innerHeight) * 2 + 1), pinch / d);
    pinch = d;
    pan(stage, dx / 2, dy / 2);
  } else if (p.button === 2 || e.shiftKey) pan(stage, dx, dy);
  else orbit(stage, dx, dy);
}

export function updateHover() {
  // the pointer left the canvas: nothing is hovered any more
  if (!mouse && app.hover) { app.hover.setHighlight(false); app.hover = null; }
  if (mouse && !app.dragging) {
    castFrom(mouse.x, mouse.y);
    const targets = [...app.agents.values()].flatMap((a) => [a.rig, a.model]);
    const hit = ray.intersectObjects(targets, true)[0];
    let agent = null;
    if (hit) { let o = hit.object; while (o && !o.userData.agent) o = o.parent; agent = o?.userData.agent || null; }
    if (agent !== app.hover) { app.hover?.setHighlight(false); app.hover = agent; agent?.setHighlight(true); }
    canvas.style.cursor = agent ? 'pointer' : 'grab';
  }
  const shown = app.hover || app.selected;
  if (shown && !app.agents.has(shown.s.id)) { app.selected = null; app.hover = null; }
  if (shown && app.agents.has(shown.s.id)) {
    const at = app.hover ? mouse : app.selectPos;
    ui.showCard(shown, at.x, at.y);
  } else if (app.selectedRegion) ui.showRegion(regionCard(app.selectedRegion), app.selectPos.x, app.selectPos.y);
  else ui.showCard(null);
}

function clearSelection() {
  app.selected = null;
  app.selectedRegion = null;
  ui.showCard(null);
  resetZoom(stage);
}

export function bindInput() {
  canvas.addEventListener('contextmenu', (e) => e.preventDefault());
  canvas.addEventListener('pointerleave', () => { if (!pointers.size) { mouse = null; if (!app.selected) ui.showCard(null); } });
  canvas.addEventListener('pointerdown', (e) => {
    canvas.setPointerCapture(e.pointerId);
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY, button: e.button });
    down = { x: e.clientX, y: e.clientY, moved: false };
    stage.vel = { az: 0, el: 0 };
    if (pointers.size === 2) { const [a, c] = [...pointers.values()]; pinch = Math.hypot(a.x - c.x, a.y - c.y); down.moved = true; }
  });
  canvas.addEventListener('pointermove', onMove);
  canvas.addEventListener('pointerup', endPointer);
  canvas.addEventListener('pointercancel', endPointer);
  canvas.addEventListener('wheel', (e) => {
    e.preventDefault();
    zoomAt(stage, new THREE.Vector2((e.clientX / innerWidth) * 2 - 1, -(e.clientY / innerHeight) * 2 + 1), Math.exp(e.deltaY * 0.0012));
  }, { passive: false });
  addEventListener('keydown', (e) => { if (e.key === 'Escape') clearSelection(); });
  const on = (id, fn) => document.getElementById(id).addEventListener('click', fn);
  on('b-home', () => resetZoom(stage));
  on('b-top', () => setPreset(stage, 'top'));
  on('b-front', () => setPreset(stage, 'front'));
  on('b-lines', (e) => { app.linesOn = !app.linesOn; e.currentTarget.classList.toggle('on', app.linesOn); });
  on('b-tags', (e) => { app.tagsAll = !app.tagsAll; e.currentTarget.classList.toggle('on', app.tagsAll); });
}
