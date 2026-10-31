// pointer, wheel, keys and buttons: orbit, pan, zoom, hover card and the camera buttons
import * as THREE from 'three';
import { zoomAt, resetZoom, orbit, pan } from './stage.js';
import { app, stage, ui, canvas } from './app.js';
import { regionInfo } from './regioncard.js';
import { isGarden } from './garden/index.js';
import { openActivity } from './activity/index.js';

// the nav manager owns the camera of every template once it runs (nav/input.js reads the same events)
// a failed nav load (stage.navFailed) hands the camera back to the plain orbit
const navOwns = () => !!stage.nav || (isGarden() && !stage.navFailed);

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
  const agents = [...app.agents.values()].map((a) => ({ id: a.s.id, label: a.isMain ? a.s.handle : a.s.role === 'team' ? a.s.name : a.s.type, team: a.s.role === 'team', busy: a.s.state === 'busy', x: a.rig.position.x, y: a.rig.position.y, z: a.rig.position.z, away: a.mode === 'away' }));
  return regionInfo(r, app.layoutJson, ui.accounts, agents);
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
      // a click on a character opens its activity log (a touch tap has no hover, so it picks here)
      const hit = app.hover || pickAgent(e.clientX, e.clientY);
      if (hit) openActivity(hit.s);
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
  if (navOwns()) return;
  if (pointers.size >= 2) {
    const [a, c] = [...pointers.values()];
    const d = Math.hypot(a.x - c.x, a.y - c.y);
    if (pinch) zoomAt(stage, new THREE.Vector2(((a.x + c.x) / 2 / innerWidth) * 2 - 1, -((a.y + c.y) / 2 / innerHeight) * 2 + 1), pinch / d);
    pinch = d;
    pan(stage, dx / 2, dy / 2);
  } else if (p.button === 2 || e.shiftKey) pan(stage, dx, dy);
  else orbit(stage, dx, dy);
}

// the agent under a screen point, or null
function pickAgent(x, y) {
  castFrom(x, y);
  const targets = [...app.agents.values()].flatMap((a) => [a.rig, a.model]);
  const hit = ray.intersectObjects(targets, true)[0];
  if (!hit) return null;
  let o = hit.object;
  while (o && !o.userData.agent) o = o.parent;
  return o?.userData.agent || null;
}

export function updateHover() {
  // the drawer shows the same session in full, so the pinned card would only cover it
  if (document.body.classList.contains('activity-open')) { ui.showCard(null); return; }
  // the pointer left the canvas: nothing is hovered any more
  if (!mouse && app.hover) { app.hover.setHighlight(false); app.hover = null; }
  if (mouse && !app.dragging) {
    const agent = pickAgent(mouse.x, mouse.y);
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
  // a managed camera is moved by the nav manager (Esc is wired in viewctl), the plain orbit goes back to its top view
  if (!navOwns()) resetZoom(stage);
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
    if (navOwns()) return;
    zoomAt(stage, new THREE.Vector2((e.clientX / innerWidth) * 2 - 1, -(e.clientY / innerHeight) * 2 + 1), Math.exp(e.deltaY * 0.0012));
  }, { passive: false });
  addEventListener('keydown', (e) => { if (e.key === 'Escape') clearSelection(); });
}
