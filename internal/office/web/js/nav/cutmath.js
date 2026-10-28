// pure math of the hall cutaway: which walls the camera looks through and how see-through they get, plus the
// hall backdrop colour. a wall is {x, z, nx, nz}: a point on it and its outward normal on the ground plane.
const RAD = Math.PI / 180;
const TILT_FULL = 50 * RAD; // at or below this tilt the walls in front are at their lowest opacity
const TILT_NONE = 62 * RAD; // at or above it they stay solid
const SOFT = 2; // distance past the wall plane over which the fade comes in

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));
const smooth = (a, b, v) => { const t = clamp((v - a) / (b - a), 0, 1); return t * t * (3 - 2 * t); };

// signed distance of the camera from the wall plane, positive on the outer side
export const wallFacing = (cam, wall) => (cam.x - wall.x) * wall.nx + (cam.z - wall.z) * wall.nz;

// opacity a wall wants for a camera {x, z, el}: low when the camera is outside it at a low tilt, else 1
export function wallOpacity({ cam, wall, low = 0.1 }) {
  const out = smooth(0, SOFT, wallFacing(cam, wall));
  const tilt = 1 - smooth(TILT_FULL, TILT_NONE, cam.el);
  return 1 - out * tilt * (1 - low);
}

// one frame of easing toward the wanted opacity
export function stepFade(cur, want, dt, reduced) {
  if (reduced) return want;
  const next = cur + (want - cur) * (1 - Math.exp(-8 * dt));
  return Math.abs(next - want) < 0.005 ? want : next;
}

const hex = (n) => '#' + n.toString(16).padStart(6, '0');
const parse = (s, fb) => { const m = /^#?([0-9a-f]{6})$/i.exec(s || ''); return parseInt(m ? m[1] : fb, 16); };
const mix = (a, b, t) => {
  let o = 0;
  for (const sh of [16, 8, 0]) o |= Math.round(((a >> sh) & 255) * (1 - t) + ((b >> sh) & 255) * t) << sh;
  return o;
};

// a calm flat colour for behind the hall: a cool light grey with a trace of the floor and wall colours
export function backdropColor(floor, walls) {
  const base = 0xd3dde2;
  return hex(mix(mix(base, parse(floor, 'bb9676'), 0.12), parse(walls, 'ffffff'), 0.1));
}
