// pure math of the sky camera: pan, tilt and height clamps, ground rays, zoom toward the cursor.
// a sky pose is {az, el, dist, target:{x,y,z}} in radians, camera = target + dir * dist (same as garden/cameras.js).
const RAD = Math.PI / 180;
export const EL_MIN = 15 * RAD;
export const EL_MAX = 89 * RAD;
export const MIN_H = 2;
export const MAX_H = 120;

export const dirOf = (az, el) => ({ x: Math.sin(az) * Math.cos(el), y: Math.sin(el), z: Math.cos(az) * Math.cos(el) });

// ground move for key input: fwd and right are -1..1 axes, az is the camera heading
export function panVector(az, { fwd, right }, speed) {
  const fx = -Math.sin(az); const fz = -Math.cos(az);
  const rx = Math.cos(az); const rz = -Math.sin(az);
  return { x: (fx * fwd + rx * right) * speed, z: (fz * fwd + rz * right) * speed };
}

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

// tilt within 15..89 degrees, camera height above the target ground within minH..maxH, target inside bounds + margin
export function clampPose(p, { bounds, margin = 6, minH = MIN_H, maxH = MAX_H, groundY = 0 } = {}) {
  const el = clamp(p.el, EL_MIN, EL_MAX);
  const s = Math.sin(el);
  const ty = p.target.y;
  const dist = clamp(p.dist, Math.max(0.1, (groundY + minH - ty) / s), Math.max(0.2, (groundY + maxH - ty) / s));
  const t = { x: p.target.x, y: ty, z: p.target.z };
  if (bounds) {
    t.x = clamp(t.x, bounds.minX - margin, bounds.maxX + margin);
    t.z = clamp(t.z, bounds.minZ - margin, bounds.maxZ + margin);
  }
  return { ...p, el, dist, target: t };
}

// world ray through a screen point (ndc -1..1, y up) for a pose with a vertical fov in degrees
export function rayFromPose(p, nx, ny, fov, aspect) {
  const d = dirOf(p.az, p.el);
  const o = { x: p.target.x + d.x * p.dist, y: p.target.y + d.y * p.dist, z: p.target.z + d.z * p.dist };
  const f = { x: -d.x, y: -d.y, z: -d.z };
  const right = { x: Math.cos(p.az), y: 0, z: -Math.sin(p.az) };
  const up = { x: f.y * right.z - f.z * right.y, y: f.z * right.x - f.x * right.z, z: f.x * right.y - f.y * right.x };
  // f x right points down for a camera looking down, so flip to get the skyward up vector
  const u = { x: -up.x, y: -up.y, z: -up.z };
  const tv = Math.tan((fov * RAD) / 2);
  const v = { x: f.x + right.x * nx * tv * aspect + u.x * ny * tv, y: f.y + u.y * ny * tv, z: f.z + right.z * nx * tv * aspect + u.z * ny * tv };
  const n = Math.hypot(v.x, v.y, v.z);
  return { o, d: { x: v.x / n, y: v.y / n, z: v.z / n } };
}

// where a ray meets the horizontal plane at height y, or null when it points away from it
export function hitGround(ray, y = 0) {
  if (Math.abs(ray.d.y) < 1e-9) return null;
  const t = (y - ray.o.y) / ray.d.y;
  if (t <= 0) return null;
  return { x: ray.o.x + ray.d.x * t, y, z: ray.o.z + ray.d.z * t };
}

// zoom by factor (below 1 = closer) keeping the ground point under the cursor where it is
export function zoomToCursor(p, factor, ndc, fov, aspect, groundY = 0) {
  const before = hitGround(rayFromPose(p, ndc.x, ndc.y, fov, aspect), groundY);
  const q = { ...p, dist: p.dist * factor, target: { ...p.target } };
  if (!before) return q;
  const after = hitGround(rayFromPose(q, ndc.x, ndc.y, fov, aspect), groundY);
  if (after) { q.target.x += before.x - after.x; q.target.z += before.z - after.z; }
  return q;
}
