// third person camera distance: pull in front of an obstacle, ease back out. pure, no three.js.
export const MIN_DIST = 0.3;
const MARGIN = 0.25;

// hit is the distance along the head-to-camera ray to the first static mesh, or null when the ray is clear
export function pullDist(want, hit, margin = MARGIN) {
  if (hit == null || hit >= want + margin) return want;
  return Math.min(want, Math.max(MIN_DIST, hit - margin));
}

// in is quick so the camera never sits inside a wall, out is slow so it does not pop. reduced motion jumps
export function easeDist(cur, target, dt, reduced) {
  if (reduced) return target;
  const k = 1 - Math.exp(-dt * (target < cur ? 25 : 2.5));
  return cur + (target - cur) * k;
}

// the camera offset from the head for a distance d out of the full one: closer means higher so the avatar and
// the aisle stay in view
export function camOffset(d, full, up = 0.8) {
  const k = Math.min(1, Math.max(0, d / full));
  return { back: d, up: up + (1 - k) * 0.5 };
}
