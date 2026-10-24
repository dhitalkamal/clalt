// eased fly-to for the sky camera: a double click point, or a preset pose. reuses the pose tween of garden/cameras.js
import { makeTween, stepTween } from '../garden/cameras.js';

export const FLY_SECONDS = 0.9;

// tween from the current sky pose to one centred on point, closer by factor, same heading and tilt
export function flyToPoint(pose, fov, point, { factor = 0.55, minDist = 6 } = {}) {
  const from = { az: pose.az, el: pose.el, dist: pose.dist, target: { ...pose.target }, fov };
  const to = { ...from, dist: Math.max(minDist, pose.dist * factor), target: { x: point.x, y: pose.target.y, z: point.z } };
  return makeTween(from, to, FLY_SECONDS);
}

// tween to a full pose {az, el, dist, target, fov}
export function flyToPose(pose, to, dur = 0.8) {
  return makeTween({ az: pose.az, el: pose.el, dist: pose.dist, target: { ...pose.target }, fov: pose.fov }, to, dur);
}

export { stepTween };
