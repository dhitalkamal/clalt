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

// the treehouse stair from walk.js stairs: foot B on the ground, top T on the deck, rise in height
export function stairFrame(st) {
  const len = Math.hypot(st.T.x - st.B.x, st.T.z - st.B.z) || 1;
  return { bx: st.B.x, bz: st.B.z, dx: st.dir[0], dz: st.dir[1], len, rise: st.rise };
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
