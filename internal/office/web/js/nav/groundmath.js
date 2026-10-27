// the maths of ground walking: headings, key intent, stair ramp, spawn. pure, no three.js.
// yaw 0 faces +z (south), yaw grows toward +x, the same as the agents' rotation.y.
export const WALK = 2.2;
export const RUN = 4.5;
export const PITCH_MIN = (-60 * Math.PI) / 180;
export const PITCH_MAX = (45 * Math.PI) / 180;

export const forwardVec = (yaw) => [Math.sin(yaw), Math.cos(yaw)];
export const rightVec = (yaw) => [-Math.cos(yaw), Math.sin(yaw)];
export const clampPitch = (p) => Math.min(PITCH_MAX, Math.max(PITCH_MIN, p));

const any = (keys, codes) => codes.some((c) => keys.has(c));

// world velocity from the held keys: W/S forward, A/D strafe, shift runs, diagonals are not faster
export function moveIntent(keys, yaw) {
  const f = (any(keys, ['KeyW', 'ArrowUp']) ? 1 : 0) - (any(keys, ['KeyS', 'ArrowDown']) ? 1 : 0);
  const s = (any(keys, ['KeyD', 'ArrowRight']) ? 1 : 0) - (any(keys, ['KeyA', 'ArrowLeft']) ? 1 : 0);
  if (!f && !s) return { vx: 0, vz: 0, moving: false, running: false };
  const running = keys.has('ShiftLeft') || keys.has('ShiftRight');
  const k = (running ? RUN : WALK) / Math.hypot(f, s);
  const [fx, fz] = forwardVec(yaw);
  const [rx, rz] = rightVec(yaw);
  return { vx: (fx * f + rx * s) * k, vz: (fz * f + rz * s) * k, moving: true, running };
}

// a stairs descriptor: foot point, unit climb direction, length along the climb, tread width, the level the top
// reaches and the rise in height. the garden treehouse and the hall stairs both provide one.
// st here is {B, T, dir, rise, width?} as made by garden/walkgrid.js stairsOf or scene/stairs.js stairWorld
export function stairDescOf(st) {
  const len = Math.hypot(st.T.x - st.B.x, st.T.z - st.B.z) || 1;
  return { foot: { x: st.B.x, z: st.B.z }, dir: [st.dir[0], st.dir[1]], len, width: st.width || 1, topLevel: 1, height: st.rise };
}

// the frame the walker uses: foot, heading, length and rise. takes a descriptor or the raw {B, T, dir, rise}
export function stairFrame(st) {
  const d = st.foot ? st : stairDescOf(st);
  return { bx: d.foot.x, bz: d.foot.z, dx: d.dir[0], dz: d.dir[1], len: d.len, rise: d.height, width: d.width, top: d.topLevel };
}

// s along the stair from the foot, lat sideways (positive to the right of the climb)
export function stairCoord(fr, x, z) {
  const ox = x - fr.bx;
  const oz = z - fr.bz;
  return { s: ox * fr.dx + oz * fr.dz, lat: ox * -fr.dz + oz * fr.dx };
}

export const stairHeight = (fr, s) => fr.rise * Math.min(1, Math.max(0, s / fr.len));

// the wanted point if walkable, else the nearest walkable point on rings around it, else the fallback
export function pickSpawn(walkable, x, z, fallback, maxR = 6, step = 0.4) {
  if (!Number.isFinite(x) || !Number.isFinite(z)) return fallback;
  if (walkable(x, z)) return { x, z };
  for (let r = step; r <= maxR; r += step) {
    const n = Math.max(8, Math.ceil((2 * Math.PI * r) / step));
    let best = null;
    let bd = Infinity;
    for (let i = 0; i < n; i++) {
      const a = (i / n) * 2 * Math.PI;
      const px = x + Math.cos(a) * r;
      const pz = z + Math.sin(a) * r;
      const d = Math.hypot(px - x, pz - z);
      if (d < bd && walkable(px, pz)) { best = { x: px, z: pz }; bd = d; }
    }
    if (best) return best;
  }
  return fallback;
}

const CLEAR = 0.8; // least distance from any agent or seat at the spawn

// where ground mode starts: the nearest walkable point to (x, z) that keeps CLEAR from every avoid point, facing
// (x, z). nothing within maxR gives the fallback (the door). yaw is used when the point is the spot itself
export function spawnPoint(walkable, x, z, { avoid = [], fallback, yaw = 0, maxR = 6, step = 0.4 }) {
  const door = { x: fallback.x, z: fallback.z, yaw, fallback: true };
  if (!Number.isFinite(x) || !Number.isFinite(z)) return door;
  const ok = (px, pz) => walkable(px, pz) && avoid.every((a) => Math.hypot(a.x - px, a.z - pz) >= CLEAR);
  const face = (px, pz) => (Math.hypot(x - px, z - pz) > 1e-3 ? Math.atan2(x - px, z - pz) : yaw);
  if (ok(x, z)) return { x, z, yaw, fallback: false };
  for (let r = step; r <= maxR; r += step) {
    const n = Math.max(8, Math.ceil((2 * Math.PI * r) / step));
    for (let i = 0; i < n; i++) {
      const a = (i / n) * 2 * Math.PI;
      const px = x + Math.cos(a) * r;
      const pz = z + Math.sin(a) * r;
      if (ok(px, pz)) return { x: px, z: pz, yaw: face(px, pz), fallback: false };
    }
  }
  return door;
}
