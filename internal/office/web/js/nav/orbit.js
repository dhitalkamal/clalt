// pure math of the free orbit: turn the sky camera around a pivot point, clamp tilt and height, shorten for walls.
// a pose is {az, el, dist, target} like skymath.js. the camera sits at target + dirOf(az, el) * dist.
import { dirOf } from './skymath.js';

const RAD = Math.PI / 180;
export const EL_LO = 3 * RAD;
export const EL_HI = 90 * RAD;
export const MIN_CAM_H = 0.6;
export const MIN_DIST = 2;
const TURN_KEY = 1.3; // rad/s on q and e
const TILT_KEY = 0.9; // rad/s on z and x

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));
const wrap = (a) => { const t = a % (2 * Math.PI); return t > Math.PI ? t - 2 * Math.PI : t <= -Math.PI ? t + 2 * Math.PI : t; };

// rotate the pose rigidly about pivot: yaw about the vertical, then pitch about the camera right axis. the pivot
// keeps its place in the camera frame, so it stays under the cursor. pitch stops at EL_LO and EL_HI.
export function orbitAround(p, pivot, dAz, dEl) {
  const el = clamp(p.el + dEl, EL_LO, EL_HI);
  const dE = el - p.el;
  const az = p.az + dAz;
  const w = { x: p.target.x - pivot.x, y: p.target.y - pivot.y, z: p.target.z - pivot.z };
  const cy = Math.cos(dAz); const sy = Math.sin(dAz);
  const x = w.x * cy + w.z * sy;
  const z = -w.x * sy + w.z * cy;
  // pitch in the plane of the horizontal toward-camera direction and up
  const hx = Math.sin(az); const hz = Math.cos(az);
  const a = x * hx + z * hz;
  const c = x * hz - z * hx;
  const a2 = a * Math.cos(dE) - w.y * Math.sin(dE);
  const b2 = a * Math.sin(dE) + w.y * Math.cos(dE);
  const t = { x: pivot.x + a2 * hx + c * hz, y: pivot.y + b2, z: pivot.z + a2 * hz - c * hx };
  return { ...p, az: wrap(az), el, target: t };
}

// tilt in 3..90 degrees, distance within the zoom range, camera height at most maxH above the ground and at
// least minCamH above it (a close low camera tilts up instead of moving back), target inside bounds + margin
export function clampOrbit(p, { bounds, margin = 6, groundY = 0, maxH = 120, minCamH = MIN_CAM_H, minDist = MIN_DIST } = {}) {
  const ty = p.target.y;
  let el = clamp(p.el, EL_LO, EL_HI);
  const maxDist = Math.min(maxH * 1.5, Math.max(minDist, (groundY + maxH - ty) / Math.sin(el)));
  const dist = clamp(p.dist, minDist, Math.max(minDist, maxDist));
  const need = (groundY + minCamH - ty) / dist;
  if (Math.sin(el) < need) el = Math.asin(clamp(need, 0, 1));
  const t = { x: p.target.x, y: ty, z: p.target.z };
  if (bounds) {
    t.x = clamp(t.x, bounds.minX - margin, bounds.maxX + margin);
    t.z = clamp(t.z, bounds.minZ - margin, bounds.maxZ + margin);
  }
  return { ...p, el, dist, target: t };
}

// hitDist is how far from the pivot a wall stands along the camera ray (null for none)
export function shortenForWall(dist, hitDist, gap = 0.5) {
  if (hitDist == null || hitDist >= dist) return dist;
  return Math.max(1, hitDist - gap);
}

// turn and tilt deltas for the held keys: q e turn, z tilts down (more top-down), x tilts up
export function keyTurn(keys, dt) {
  const n = (k) => (keys.has(k) ? 1 : 0);
  return { dAz: (n('KeyE') - n('KeyQ')) * TURN_KEY * dt, dEl: (n('KeyZ') - n('KeyX')) * TILT_KEY * dt };
}

// release momentum of the drag: exponential decay, zero under reduced motion or when it is tiny
export function stepMomentum(v, dt, reduced) {
  if (reduced) return { yaw: 0, pitch: 0 };
  const k = Math.exp(-4.5 * dt);
  const yaw = v.yaw * k; const pitch = v.pitch * k;
  return Math.abs(yaw) + Math.abs(pitch) < 1e-3 ? { yaw: 0, pitch: 0 } : { yaw, pitch };
}

export { dirOf };
