// camera presets for the garden: top, plan, iso, hero, eye, orbit. pure math, no three.js, so it runs in node tests.
// a pose is {az, el, dist, target:{x,y,z}, fov, pos:{x,y,z}} with angles in radians and fov in degrees. the camera
// sits at target + dir * dist, dir = (sin az cos el, sin el, cos az cos el), az 0 looks from the south, which is
// the same convention as stage.view on the page. glue to the page camera lives in garden/index.js.
const RAD = Math.PI / 180;
export const PRESET_IDS = ['top', 'plan', 'iso', 'hero', 'eye', 'orbit'];

// angles in degrees as written in the plan: top is 75 degrees, plan is straight down
export const PRESETS = {
  top: { az: 0, el: 75, fov: 30 },
  plan: { az: 0, el: 89.6, fov: 30 },
  iso: { az: 45, el: 35, fov: 10 },
  hero: { az: -26, el: 25, fov: 34 },
  eye: { fov: 58 },
  orbit: { az: 28, el: 42, fov: 30 },
};
export const EYE_Y = 0.95;
export const EYE_LOOK_Y = 0.55;
export const ORBIT_SPEED = 0.12; // radians per second while auto-spinning

const dirOf = (az, el) => ({ x: Math.sin(az) * Math.cos(el), y: Math.sin(el), z: Math.cos(az) * Math.cos(el) });
const dot = (a, b) => a.x * b.x + a.y * b.y + a.z * b.z;

// camera basis for az and el: right and up as unit vectors, dir points from the target to the camera
function basis(az, el) {
  const dir = dirOf(az, el);
  const right = { x: Math.cos(az), y: 0, z: -Math.sin(az) };
  // up = dir x right, perpendicular to both and pointing skyward
  const up = { x: dir.y * right.z - dir.z * right.y, y: dir.z * right.x - dir.x * right.z, z: dir.x * right.y - dir.y * right.x };
  return { dir, right, up };
}

const withPos = (p) => {
  const d = dirOf(p.az, p.el);
  return { ...p, pos: { x: p.target.x + d.x * p.dist, y: p.target.y + d.y * p.dist, z: p.target.z + d.z * p.dist } };
};

// corners of the box the camera must show: the island plus a little air above and below it
const corners = (bb) => {
  const out = [];
  for (const x of [bb.x0, bb.x1]) for (const y of [bb.bottom ?? -1.2, bb.top ?? 3.4]) for (const z of [bb.z0, bb.z1]) out.push({ x, y, z });
  return out;
};

// ndc of a world point seen through a pose. safe margins are not part of the projection
export function project(pose, p, aspect) {
  const { dir, right, up } = basis(pose.az, pose.el);
  const tv = Math.tan((pose.fov * RAD) / 2);
  const q = { x: p.x - pose.target.x, y: p.y - pose.target.y, z: p.z - pose.target.z };
  const depth = pose.dist - dot(q, dir);
  return { x: dot(q, right) / (depth * tv * aspect), y: dot(q, up) / (depth * tv), depth };
}

// distance and target that fit bb inside the screen minus the safe margins (fractions 0..1 of the screen)
export function fitBounds(bb, { az, el, fov, aspect = 16 / 9, safe = {} }) {
  const { dir, right, up } = basis(az * RAD, el * RAD);
  const l = safe.left || 0;
  const r = safe.right || 0;
  const t = safe.top || 0;
  const b = safe.bottom || 0;
  const hx = (1 - l - r) * 0.97;
  const hy = (1 - t - b) * 0.97;
  const cx = l - r;
  const cy = b - t;
  const tv = Math.tan((fov * RAD) / 2);
  const pts = corners(bb);
  const c0 = { x: (bb.x0 + bb.x1) / 2, y: 0.6, z: (bb.z0 + bb.z1) / 2 };
  // at a fixed distance, slide the target until the projected extents centre on the safe rect. the
  // slide is in the image plane so depths hold, but perspective makes it uneven: iterate it
  const settle = (dist) => {
    const c = { ...c0 };
    let ext;
    for (let it = 0; it < 8; it++) {
      ext = { x0: Infinity, x1: -Infinity, y0: Infinity, y1: -Infinity };
      for (const p of pts) {
        const n = project({ az: az * RAD, el: el * RAD, dist, target: c, fov }, p, aspect);
        ext.x0 = Math.min(ext.x0, n.x); ext.x1 = Math.max(ext.x1, n.x); ext.y0 = Math.min(ext.y0, n.y); ext.y1 = Math.max(ext.y1, n.y);
      }
      const sx = ((ext.x0 + ext.x1) / 2 - cx) * dist * tv * aspect;
      const sy = ((ext.y0 + ext.y1) / 2 - cy) * dist * tv;
      c.x += right.x * sx + up.x * sy;
      c.y += right.y * sx + up.y * sy;
      c.z += right.z * sx + up.z * sy;
    }
    return { c, fits: ext.x1 - ext.x0 <= 2 * hx + 1e-4 && ext.y1 - ext.y0 <= 2 * hy + 1e-4 };
  };
  // smallest distance whose centred extents fit: extents shrink as the camera backs off, so bisect.
  // the depth of the deepest corner is the lower bound (nothing may sit behind the camera)
  let lo = 0;
  for (const p of pts) lo = Math.max(lo, dot({ x: p.x - c0.x, y: p.y - c0.y, z: p.z - c0.z }, dir));
  let hi = Math.max(lo * 2, 40);
  while (!settle(hi).fits && hi < 1e5) hi *= 2;
  for (let it = 0; it < 40; it++) {
    const mid = (lo + hi) / 2;
    if (settle(mid).fits) hi = mid; else lo = mid;
  }
  return { dist: hi, target: { ...settle(hi).c } };
}

// pose from a camera position and the point it looks at
export function poseFromLook(pos, look, fov) {
  const d = { x: pos.x - look.x, y: pos.y - look.y, z: pos.z - look.z };
  const dist = Math.hypot(d.x, d.y, d.z) || 1;
  return withPos({ az: Math.atan2(d.x, d.z), el: Math.asin(d.y / dist), dist, target: { ...look }, fov });
}

// the eye level view: the layout's eye point, else a spot on the east side looking at the middle
export function eyePose(bb, eye) {
  const from = eye ? eye.from : [bb.x0 + (bb.x1 - bb.x0) * 0.72, bb.z0 + (bb.z1 - bb.z0) * 0.62];
  const look = eye ? eye.look : [(bb.x0 + bb.x1) / 2, (bb.z0 + bb.z1) / 2];
  return poseFromLook({ x: from[0], y: EYE_Y, z: from[1] }, { x: look[0], y: EYE_LOOK_Y, z: look[1] }, PRESETS.eye.fov);
}

// the pose of one preset for the island bounds. opts: {bb, aspect, safe, eye}
export function presetPose(id, { bb, aspect = 16 / 9, safe, eye }) {
  if (id === 'eye') return eyePose(bb, eye);
  const p = PRESETS[id] || PRESETS.top;
  const fit = fitBounds(bb, { az: p.az, el: p.el, fov: p.fov, aspect, safe });
  return withPos({ az: p.az * RAD, el: p.el * RAD, dist: fit.dist, target: fit.target, fov: p.fov });
}

// fog and camera far plane for a camera distance: far presets (iso at fov 10 sits far back) must not sit in haze
export function fogRange(dist) {
  return { near: Math.max(70, dist * 1.9), far: Math.max(220, dist * 6), camFar: Math.max(400, dist * 8) };
}

const ease = (t) => t * t * (3 - 2 * t);
const wrap = (a) => Math.atan2(Math.sin(a), Math.cos(a));

// a tween between two poses. angles take the short way round, distance moves on a log scale
export function makeTween(from, to, dur = 0.8) {
  return { from, to, dur, t: 0, done: false };
}

export function stepTween(tw, dt) {
  tw.t = Math.min(1, tw.t + dt / tw.dur);
  const k = ease(tw.t);
  const { from, to } = tw;
  const lerp = (a, b) => a + (b - a) * k;
  const out = {
    az: from.az + wrap(to.az - from.az) * k,
    el: lerp(from.el, to.el),
    dist: Math.exp(lerp(Math.log(from.dist), Math.log(to.dist))),
    target: { x: lerp(from.target.x, to.target.x), y: lerp(from.target.y, to.target.y), z: lerp(from.target.z, to.target.z) },
    fov: lerp(from.fov, to.fov),
  };
  tw.done = tw.t >= 1;
  return withPos(out);
}

// glue between these pure presets and a page camera. a stage carries a rig made by garden/index.js:
// {bb, eye, safe, aspect(), view(), apply(pose), id, tween, spin}. nothing here touches three.js.
export function goTo(stage, id, { instant = false } = {}) {
  const rig = stage.rig;
  if (!rig || !PRESET_IDS.includes(id)) return false;
  rig.id = id;
  // with the navigation manager running it flies the camera, this file only supplies the pose
  if (stage.nav) { stage.nav.toPreset(id, { instant }); return true; }
  const to = presetPose(id, { bb: rig.bb, aspect: rig.aspect(), safe: rig.safe, eye: rig.eye });
  if (instant) { rig.tween = null; rig.apply(to); return true; }
  rig.tween = makeTween(rig.view(), to, 0.8);
  return true;
}

export function setSpin(stage, on) {
  if (stage.rig) stage.rig.spin = !!on;
}

export const currentId = (stage) => (stage.rig ? stage.rig.id : null);

// reserve screen space for panels, fractions of the screen. refits the current preset
export function setSafe(stage, safe) {
  if (!stage.rig) return;
  stage.rig.safe = safe;
  // a free camera must not jump when the window resizes, an untouched preset refits
  if (stage.nav) { if (stage.nav.refit) stage.nav.refit(); return; }
  goTo(stage, stage.rig.id, { instant: true });
}

// once per frame: runs a tween, spins the orbit view, gives way to the user dragging
export function cameraTick(stage, dt, dragging) {
  const rig = stage.rig;
  if (!rig) return;
  if (stage.nav) { stage.nav.update(dt); return; }
  if (dragging) rig.tween = null;
  if (rig.tween) {
    rig.apply(stepTween(rig.tween, dt));
    if (rig.tween.done) rig.tween = null;
  } else if (rig.spin && rig.id === 'orbit' && !dragging) {
    const v = rig.view();
    rig.apply({ ...v, az: v.az + ORBIT_SPEED * dt });
  }
}
