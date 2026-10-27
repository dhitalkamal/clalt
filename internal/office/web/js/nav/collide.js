// sliding collision of a walker (a disc) on a walkable(x, z) test. pure, no three.js.
export const RADIUS = 0.25;
const MAX_STEP = 0.08; // under half a grid cell and under the thinnest wall, so a step cannot skip a blocker
const DIRS = Array.from({ length: 8 }, (_, i) => [Math.cos((i * Math.PI) / 4), Math.sin((i * Math.PI) / 4)]);

// the centre and eight points on the rim must all be walkable
export function circleFree(x, z, r, walkable) {
  if (!walkable(x, z)) return false;
  for (const [dx, dz] of DIRS) if (!walkable(x + dx * r, z + dz * r)) return false;
  return true;
}

// moves p by (dx, dz) in small steps. a blocked step slides along the open axis. when the start is already too
// tight for the disc (spawn beside a wall) only the centre is tested so the walker can always get out
export function moveSlide(p, dx, dz, walkable, radius = RADIUS) {
  const n = Math.max(1, Math.ceil(Math.hypot(dx, dz) / MAX_STEP));
  const sx = dx / n;
  const sz = dz / n;
  let x = p.x;
  let z = p.z;
  for (let i = 0; i < n; i++) {
    const tight = !circleFree(x, z, radius, walkable);
    const ok = (a, b) => (tight ? walkable(a, b) : circleFree(a, b, radius, walkable));
    if (ok(x + sx, z + sz)) { x += sx; z += sz; } else if (sx && ok(x + sx, z)) x += sx;
    else if (sz && ok(x, z + sz)) z += sz;
  }
  return { x, z };
}
