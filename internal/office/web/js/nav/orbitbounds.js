// pure bounds for the sky camera, the same for every template: the target stays inside the walkable box, the
// distance never goes past the point where the box fills the view, and a low camera stays inside the box
// (the hall walls, the island edge). a pose is {az, el, dist, target} like skymath.js, bounds {minX, maxX, minZ, maxZ}.
import { dirOf } from './skymath.js';

const RAD = Math.PI / 180;
const LOW_EL = 25 * RAD; // below this the camera is held fully inside the box
const HIGH_EL = 50 * RAD; // from here up only the fitted distance limits it
const INSET = 0.4;
const MIN_DIST = 2;
const SNAP = 1e-3;

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));
const smooth = (a, b, v) => { const t = clamp((v - a) / (b - a), 0, 1); return t * t * (3 - 2 * t); };

// k below 1 moves only part of the way to the edge per call, so a drag past the edge settles without a snap
// the target never sits below the walkable surface (a tween can leave it under the island)
export function clampTarget(t, bounds, k = 1, minY = 0) {
  const y = Math.max(t.y, minY);
  if (!bounds) return { ...t, y };
  const to = (v, lo, hi) => {
    const goal = clamp(v, lo, hi);
    return Math.abs(goal - v) < SNAP || k >= 1 ? goal : v + (goal - v) * k;
  };
  return { x: to(t.x, bounds.minX, bounds.maxX), y, z: to(t.z, bounds.minZ, bounds.maxZ) };
}

// a pivot farther than reach from the target (a ray near the horizon lands far away) is replaced by the target
export function limitPivot(pivot, target, reach) {
  if (!pivot) return { ...target };
  const far = Math.hypot(pivot.x - target.x, pivot.y - target.y, pivot.z - target.z) > reach;
  return far ? { ...target } : pivot;
}

// distance at which the box (with some wall height) just fits the view for this heading and tilt, times a slack
// that leaves room for the page chrome. the target is taken at the box centre.
export function maxDistFor({ bounds, az, el, fov, aspect, height = 2.6, slack = 1.35 }) {
  const w = (bounds.maxX - bounds.minX) / 2;
  const d = (bounds.maxZ - bounds.minZ) / 2;
  const ca = Math.abs(Math.cos(az)); const sa = Math.abs(Math.sin(az));
  const across = ca * w + sa * d;
  const depth = sa * w + ca * d;
  const tv = Math.tan((fov * RAD) / 2);
  const up = depth * Math.sin(el) + (height / 2) * Math.cos(el);
  return Math.max(across / (tv * aspect), up / tv) * slack;
}

// largest distance that keeps the camera inside the box (inset from the walls) along the horizontal heading
function insideDist(target, az, el, bounds, inset) {
  const d = dirOf(az, el);
  let cap = Infinity;
  const axis = (dv, t, lo, hi) => {
    if (Math.abs(dv) < 1e-9) return;
    cap = Math.min(cap, dv > 0 ? (hi - inset - t) / dv : (lo + inset - t) / dv);
  };
  axis(d.x, target.x, bounds.minX, bounds.maxX);
  axis(d.z, target.z, bounds.minZ, bounds.maxZ);
  return cap;
}

// the distance limit: inside the box when low, the fitted maximum when steep, smooth in between
export function camDistCap({ target, az, el, bounds, maxDist, inset = INSET }) {
  if (!bounds) return maxDist;
  // straight down there is no heading, insideDist is Infinity, so cap it or the blend below is Infinity - Infinity
  const inside = Math.max(MIN_DIST, Math.min(maxDist, insideDist(target, az, el, bounds, inset)));
  const s = smooth(LOW_EL, HIGH_EL, el);
  return Math.min(maxDist, inside + (maxDist - inside) * s);
}

// target clamped (k = settle factor), distance capped, and a low camera pushed back inside by moving the target
export function boundPose(p, { bounds, fov, aspect, height, slack, k = 1 }) {
  const target = clampTarget(p.target, bounds, k);
  if (!bounds) return target.y === p.target.y ? p : { ...p, target };
  const maxDist = maxDistFor({ bounds, az: p.az, el: p.el, fov, aspect, height, slack });
  const dist = Math.max(MIN_DIST, Math.min(p.dist, camDistCap({ target, az: p.az, el: p.el, bounds, maxDist })));
  const q = { ...p, dist, target };
  const hold = 1 - smooth(LOW_EL, HIGH_EL, p.el);
  if (hold > 0) {
    const d = dirOf(p.az, p.el);
    const cx = target.x + d.x * dist; const cz = target.z + d.z * dist;
    const ex = cx - clamp(cx, bounds.minX + INSET, bounds.maxX - INSET);
    const ez = cz - clamp(cz, bounds.minZ + INSET, bounds.maxZ - INSET);
    if (ex || ez) q.target = clampTarget({ x: target.x - ex * hold, y: target.y, z: target.z - ez * hold }, bounds);
  }
  return q;
}
