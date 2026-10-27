// 2d overlay drawn above the 3d canvas: grid, rooms, furniture, selection,
// ghost and red problem highlights. everything is projected through the camera.
import { furnitureRect, handlePoint, HANDLES, rotateFootprint } from './geometry.js';
import { canPlace } from './picking.js';

export const TYPE_COLOR = {
  lobby: '#e0a63c', cabins: '#4f8fd8', research: '#8d6fd1', workspace: '#4caf7d',
  review: '#d1624f', meeting: '#d16fa8', cafe: '#b58a5b', play: '#38b6c7',
};
const RED = '#e5372e';
const AMBER = '#d99a00';
const SEL = '#1d6fe0';
// facing of rot 0 is +z; a quarter turn goes toward +x (three.js rotation.y)
const FACING = { 0: [0, 1], 90: [1, 0], 180: [0, -1], 270: [-1, 0] };

function path(ctx, v, level, r) {
  const pts = [[r.x, r.z], [r.x + r.w, r.z], [r.x + r.w, r.z + r.d], [r.x, r.z + r.d]].map(([x, z]) => v.project(level, x, z));
  ctx.beginPath();
  pts.forEach((p, i) => (i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y)));
  ctx.closePath();
  return pts;
}

function line(ctx, a, b) {
  ctx.moveTo(a.x, a.y);
  ctx.lineTo(b.x, b.y);
}

function drawGrid(ctx, ed, v) {
  const { w, d } = ed.layout.size;
  ctx.lineWidth = 1;
  for (let i = 0; i <= w; i++) {
    ctx.strokeStyle = i % 4 === 0 ? 'rgba(30,40,60,.34)' : 'rgba(30,40,60,.14)';
    ctx.beginPath();
    line(ctx, v.project(0, i, 0), v.project(0, i, d));
    ctx.stroke();
  }
  for (let j = 0; j <= d; j++) {
    ctx.strokeStyle = j % 4 === 0 ? 'rgba(30,40,60,.34)' : 'rgba(30,40,60,.14)';
    ctx.beginPath();
    line(ctx, v.project(0, 0, j), v.project(0, w, j));
    ctx.stroke();
  }
  ctx.strokeStyle = 'rgba(30,40,60,.7)';
  ctx.lineWidth = 2;
  path(ctx, v, 0, { x: 0, z: 0, w, d });
  ctx.stroke();
}

function drawDoor(ctx, ed, v) {
  const { side, at } = ed.layout.door || {};
  const { w, d } = ed.layout.size;
  const seg = { left: [0, at, 0, at + 1], right: [w, at, w, at + 1], back: [at, 0, at + 1, 0], front: [at, d, at + 1, d] }[side];
  if (!seg) return;
  ctx.strokeStyle = ed.result.errors.some((i) => i.code === 'door-range') ? RED : '#2b2433';
  ctx.lineWidth = 6;
  ctx.beginPath();
  line(ctx, v.project(0, seg[0], seg[1]), v.project(0, seg[2], seg[3]));
  ctx.stroke();
}

function label(ctx, text, x, y, color) {
  ctx.font = '600 11px system-ui, sans-serif';
  ctx.textAlign = 'center';
  ctx.fillStyle = 'rgba(255,255,255,.85)';
  const wid = ctx.measureText(text).width + 8;
  ctx.fillRect(x - wid / 2, y - 8, wid, 15);
  ctx.fillStyle = color;
  ctx.fillText(text, x, y + 3);
}

function seatText(ed, room) {
  const seats = ed.seats.get(room.id) || 0;
  const need = typeof ed.opts.sessionCount === 'function' ? ed.opts.sessionCount(room) : ed.opts.sessionCount;
  if (room.type === 'cabins' && need != null) return seats + '/' + need + ' seats';
  return seats ? seats + (seats === 1 ? ' seat' : ' seats') : '';
}

function drawRoom(ctx, ed, v, r, dim) {
  const bad = ed.result.bad.has(r.id);
  const warn = ed.result.warn.has(r.id);
  const color = TYPE_COLOR[r.type] || '#888';
  ctx.globalAlpha = dim ? 0.35 : 1;
  const pts = path(ctx, v, r.level, r);
  ctx.fillStyle = bad ? 'rgba(229,55,46,.28)' : color + '40';
  ctx.fill();
  ctx.lineWidth = bad ? 3 : warn ? 2.5 : 1.5;
  ctx.strokeStyle = bad ? RED : warn ? AMBER : color;
  ctx.stroke();
  const c = v.project(r.level, r.x + r.w / 2, r.z + r.d / 2);
  if (c.visible && Math.hypot(pts[2].x - pts[0].x, pts[2].y - pts[0].y) > 60) {
    label(ctx, r.label, c.x, c.y - 8, bad ? RED : '#2b2433');
    const t = seatText(ed, r);
    if (t) label(ctx, t, c.x, c.y + 10, warn ? AMBER : '#4a4255');
  }
  ctx.globalAlpha = 1;
}

function drawFurniture(ctx, ed, v, f, dim) {
  const rect = furnitureRect(f, ed.catMap);
  if (!rect) return;
  const bad = ed.result.bad.has(f.id);
  ctx.globalAlpha = dim ? 0.35 : 1;
  const pts = path(ctx, v, f.level, rect);
  ctx.fillStyle = bad ? 'rgba(229,55,46,.55)' : 'rgba(255,255,255,.78)';
  ctx.fill();
  ctx.lineWidth = bad ? 3 : 1.2;
  ctx.strokeStyle = bad ? RED : 'rgba(40,30,50,.8)';
  ctx.stroke();
  // facing tick from the center toward the front side
  const [fx, fz] = FACING[f.rot || 0];
  const c = v.project(f.level, rect.x + rect.w / 2, rect.z + rect.d / 2);
  const tip = v.project(f.level, rect.x + rect.w / 2 + fx * rect.w * 0.45, rect.z + rect.d / 2 + fz * rect.d * 0.45);
  ctx.beginPath();
  line(ctx, c, tip);
  ctx.stroke();
  if (Math.hypot(pts[2].x - pts[0].x, pts[2].y - pts[0].y) > 46 && c.visible) {
    ctx.font = '10px system-ui, sans-serif';
    ctx.textAlign = 'center';
    ctx.fillStyle = '#2b2433';
    ctx.fillText(f.item, c.x, c.y + 3);
  }
  ctx.globalAlpha = 1;
}

function drawSelection(ctx, ed, v) {
  const room = ed.selRoom();
  const furn = ed.selFurn();
  const rect = room || (furn && furnitureRect(furn, ed.catMap));
  if (!rect) return;
  ctx.strokeStyle = SEL;
  ctx.lineWidth = 3;
  ctx.setLineDash([6, 4]);
  path(ctx, v, room ? room.level : furn.level, rect);
  ctx.stroke();
  ctx.setLineDash([]);
  if (!room || room.level !== 0) return;
  for (const p of handleScreen(ed, v)) {
    ctx.fillStyle = '#fff';
    ctx.strokeStyle = SEL;
    ctx.lineWidth = 2;
    ctx.fillRect(p.x - 5, p.y - 5, 10, 10);
    ctx.strokeRect(p.x - 5, p.y - 5, 10, 10);
  }
}

// screen positions of the resize handles of the selected room
export function handleScreen(ed, v) {
  const room = ed.selRoom();
  if (!room || room.level !== 0) return [];
  return HANDLES.map((h) => {
    const p = handlePoint(room, h);
    return { h, ...v.project(0, p.x, p.z) };
  });
}

function drawGhost(ctx, ed, v) {
  if (ed.tool === 'furniture' && ed.furnItem && ed.hover) {
    const c = ed.catMap.get(ed.furnItem);
    if (!c) return;
    const fp = rotateFootprint(c.footprint, ed.ghostRot);
    const x = ed.hover.x - Math.floor(fp.w / 2);
    const z = ed.hover.z - Math.floor(fp.d / 2);
    const ok = canPlace(ed.layout, ed.catMap, ed.furnItem, ed.ghostRot, 0, x, z);
    path(ctx, v, 0, { x, z, w: fp.w, d: fp.d });
    ctx.fillStyle = ok ? 'rgba(29,111,224,.3)' : 'rgba(229,55,46,.4)';
    ctx.fill();
    ctx.strokeStyle = ok ? SEL : RED;
    ctx.lineWidth = 2;
    ctx.stroke();
  }
  if (ed.dragRect) {
    path(ctx, v, 0, ed.dragRect);
    ctx.fillStyle = (TYPE_COLOR[ed.roomType] || '#888') + '55';
    ctx.fill();
    ctx.setLineDash([6, 4]);
    ctx.strokeStyle = SEL;
    ctx.lineWidth = 2;
    ctx.stroke();
    ctx.setLineDash([]);
  }
}

export function drawOverlay(canvas, ed, v) {
  const s = v.size();
  const dpr = window.devicePixelRatio || 1;
  const pw = Math.round(s.w * dpr);
  const ph = Math.round(s.h * dpr);
  if (canvas.width !== pw || canvas.height !== ph) {
    canvas.width = pw;
    canvas.height = ph;
  }
  const ctx = canvas.getContext('2d');
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, s.w, s.h);
  v.refresh();
  drawGrid(ctx, ed, v);
  // upper level is read only: dimmed, never hit
  ed.layout.rooms.filter((r) => r.level !== 0).forEach((r) => drawRoom(ctx, ed, v, r, true));
  ed.layout.furniture.filter((f) => f.level !== 0).forEach((f) => drawFurniture(ctx, ed, v, f, true));
  ed.layout.rooms.filter((r) => r.level === 0).forEach((r) => drawRoom(ctx, ed, v, r, false));
  ed.layout.furniture.filter((f) => f.level === 0).forEach((f) => drawFurniture(ctx, ed, v, f, false));
  drawDoor(ctx, ed, v);
  drawSelection(ctx, ed, v);
  drawGhost(ctx, ed, v);
}
