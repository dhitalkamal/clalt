// the sky controller: map-app camera over the island. left drag orbits around the point under the cursor (free yaw,
// tilt 3..90 degrees), shift+left or right drag grabs the ground, wasd pan, q/e turn, z/x tilt, wheel zooms toward
// the cursor, r/pageup rise, c/pagedown lower, double click flies to a point.
import { poseFromLook } from '../garden/cameras.js';
import { panVector, rayFromPose, hitGround, zoomToCursor, dirOf, MAX_H } from './skymath.js';
import { orbitAround, clampOrbit, keyTurn, shortenForWall, stepMomentum } from './orbit.js';
import { dragMode } from './input.js';
import { createPicker } from './skypick.js';
import { boundPose } from './orbitbounds.js';
import { flyToPoint, flyToPose, stepTween } from './flyto.js';

const down = (k, ...codes) => codes.some((c) => k.has(c));
const damp = (rate, dt) => 1 - Math.exp(-rate * dt);

export function createSky() {
  let p = { az: 0, el: 1, dist: 40, target: { x: 0, y: 0, z: 0 } };
  let fov = 30;
  let maxH = MAX_H;
  let vel = { x: 0, z: 0 };
  let zoomLog = 0;
  let zoomNdc = { x: 0, y: 0 };
  let grab = null;
  let drag = null; // {mode, pivot} of the held mouse button, latched on press
  let spinV = { yaw: 0, pitch: 0 }; // drag momentum, rad/s
  let eff = 40; // camera distance after the wall shortening
  let wallT = 0;
  let wallAt = null;
  let wallHit = null;
  let picker = null;
  let lastPivot = null;
  let tween = null;
  let freed = false;
  const c = { name: 'sky', spin: false };

  const size = (ctx) => {
    const d = ctx.stage.renderer && ctx.stage.renderer.domElement;
    return { w: (d && d.clientWidth) || innerWidth, h: (d && d.clientHeight) || innerHeight };
  };
  const limits = (ctx) => ({ bounds: ctx.garden && ctx.garden.bounds, maxH });
  const groundAt = (ctx, x, z) => (ctx.garden && ctx.garden.groundY ? ctx.garden.groundY(x, z) : 0);
  const camXZ = () => { const d = dirOf(p.az, p.el); return { x: p.target.x + d.x * p.dist, z: p.target.z + d.z * p.dist }; };

  function apply(ctx) {
    const { camera, controls } = ctx.stage;
    const d = dirOf(p.az, p.el);
    camera.position.set(p.target.x + d.x * eff, p.target.y + d.y * eff, p.target.z + d.z * eff);
    // straight down has no horizon to point up at, so use the heading as the up vector
    if (p.el > 1.5698) camera.up.set(-Math.sin(p.az), 0, -Math.cos(p.az)); else camera.up.set(0, 1, 0);
    if (camera.fov !== fov) { camera.fov = fov; camera.updateProjectionMatrix(); }
    if (controls && controls.target && controls.target.set) controls.target.set(p.target.x, p.target.y, p.target.z);
    camera.lookAt(p.target.x, p.target.y, p.target.z);
    camera.updateMatrixWorld();
    // keep the page's view record (zoom limits, hall buttons) in step with the camera
    const v = ctx.stage.view;
    if (v && v.target && v.target.set) { v.az = p.az; v.el = p.el; v.dist = p.dist; v.target.set(p.target.x, p.target.y, p.target.z); }
    if (ctx.fitFog) ctx.fitFog(p.dist);
  }

  function touch(ctx) {
    if (!freed) { freed = true; if (ctx.onFree) ctx.onFree(); }
    tween = null;
  }

  const ndcOf = (ctx, x, y) => { const s = size(ctx); return { x: (x / s.w) * 2 - 1, y: -(y / s.h) * 2 + 1 }; };
  const rayAt = (ctx, x, y) => {
    const n = ndcOf(ctx, x, y); const s = size(ctx);
    return rayFromPose(p, n.x, n.y, fov, s.w / s.h);
  };

  c.enter = (ctx, from) => {
    picker = createPicker(ctx);
    const cam = ctx.stage.camera;
    fov = cam.fov || 30;
    const ct = ctx.stage.controls && ctx.stage.controls.target;
    const f = from || { pos: [cam.position.x, cam.position.y, cam.position.z], target: ct ? [ct.x, ct.y, ct.z] : [0, 0, 0] };
    const q = poseFromLook({ x: f.pos[0], y: f.pos[1], z: f.pos[2] }, { x: f.target[0], y: f.target[1], z: f.target[2] }, fov);
    p = { az: q.az, el: q.el, dist: q.dist, target: { x: q.target.x, y: 0, z: q.target.z } };
    // coming from a low view (ground, follow): rise above the spot instead of staying at head height
    if (q.dist < 25) { p.el = Math.max(p.el, 0.9); p.dist = 35; p.target.x = f.pos[0]; p.target.z = f.pos[2]; }
    p = clampOrbit(p, { ...limits(ctx), maxH: Math.max(MAX_H, p.dist * Math.sin(p.el) * 1.1) });
    eff = p.dist;
    apply(ctx);
  };

  // fly to a preset pose {az, el, dist, target, fov}. a wide preset lifts the height cap so it is not clipped
  c.flyToPreset = (ctx, to, instant) => {
    maxH = Math.max(MAX_H, to.dist * Math.sin(to.el) * 1.15);
    freed = false;
    vel = { x: 0, z: 0 }; zoomLog = 0;
    if (instant) { p = { az: to.az, el: to.el, dist: to.dist, target: { ...to.target } }; fov = to.fov; tween = null; eff = p.dist; apply(ctx); return; }
    tween = flyToPose({ ...p, fov }, to);
  };

  // camera distance after a wall between the pivot and the camera: one ray, at most every 70 ms, eased back out
  function wallDistance(ctx, dt) {
    const key = [p.az, p.el, p.dist, p.target.x, p.target.y, p.target.z].map((v) => v.toFixed(3)).join();
    wallT -= dt;
    if (key !== wallAt && wallT <= 0 && picker) {
      wallAt = key; wallT = 0.07;
      // steep views look over props, and a prop right at the pivot is not a wall: start 2 units out, only below 50 degrees
      const d = dirOf(p.az, p.el);
      const o = { x: p.target.x + d.x * 2, y: Math.max(0.3, p.target.y + d.y * 2 + 0.3), z: p.target.z + d.z * 2 };
      const h = p.el < 0.87 && p.dist > 2 ? picker.wallAlong(o, d, p.dist - 2) : null;
      wallHit = h == null ? null : h + 2;
    }
    const goal = shortenForWall(p.dist, wallHit);
    if (goal < eff || ctx.reducedMotion) return goal;
    return Math.min(goal, eff + (goal - eff) * damp(5, dt));
  }

  c.update = (ctx, dt, input) => {
    const k = input.keys;
    const m = input.mouse;
    let moved = false;
    const s = size(ctx);
    const aspect = s.w / s.h;

    // keys: pan with damping, rotate, rise and lower
    const fwd = (down(k, 'KeyW', 'ArrowUp') ? 1 : 0) - (down(k, 'KeyS', 'ArrowDown') ? 1 : 0);
    const right = (down(k, 'KeyD', 'ArrowRight') ? 1 : 0) - (down(k, 'KeyA', 'ArrowLeft') ? 1 : 0);
    const speed = p.dist * 0.9 * (down(k, 'ShiftLeft', 'ShiftRight') ? 2.5 : 1);
    const want = panVector(p.az, { fwd, right }, speed);
    const kd = damp(10, dt);
    vel.x += (want.x - vel.x) * kd; vel.z += (want.z - vel.z) * kd;
    if (!(drag && drag.mode === 'pan') && Math.abs(vel.x) + Math.abs(vel.z) > 1e-3) { p.target.x += vel.x * dt; p.target.z += vel.z * dt; moved = true; }
    // q e turn and z x tilt about the point in the middle of the screen
    const kt = keyTurn(k, dt);
    if (kt.dAz || kt.dEl) { p = orbitAround(p, p.target, kt.dAz, kt.dEl); moved = true; }
    const lift = (down(k, 'KeyR', 'PageUp') ? 1 : 0) - (down(k, 'KeyC', 'PageDown') ? 1 : 0);
    if (lift) { p.dist *= Math.exp(lift * 1.2 * dt); moved = true; }

    // a press picks the mode (left orbit, shift+left or right pan) and the pivot, and keeps them until release
    const want2 = dragMode(m.buttons, m.shift, m.ctrl);
    if (!want2) {
      if (ctx.reducedMotion) spinV = { yaw: 0, pitch: 0 };
      drag = null; grab = null;
    } else if (!drag) {
      drag = { mode: want2, pivot: null };
      spinV = { yaw: 0, pitch: 0 };
      const ray = rayAt(ctx, m.x, m.y);
      if (want2 === 'pan') grab = hitGround(ray, 0) || false;
      else {
        const sp = picker.pivotAt(ray, p.dist * 3);
        const gp = hitGround(ray, groundAt(ctx, p.target.x, p.target.z));
        drag.pivot = sp || gp || { ...p.target };
        lastPivot = drag.pivot;
      }
    }
    if (drag && drag.mode === 'pan' && grab && (m.dx || m.dy)) {
      const h = hitGround(rayAt(ctx, m.x, m.y), 0);
      if (h) { p.target.x += grab.x - h.x; p.target.z += grab.z - h.z; vel = { x: (grab.x - h.x) / Math.max(dt, 1e-3) * 0.5, z: (grab.z - h.z) / Math.max(dt, 1e-3) * 0.5 }; moved = true; }
    }
    if (drag && drag.mode === 'orbit') {
      const dAz = -m.dx * 0.005; const dEl = m.dy * 0.004;
      if (dAz || dEl) { p = orbitAround(p, drag.pivot, dAz, dEl); moved = true; }
      // smoothed release speed: a pause before letting go leaves none
      const kv = damp(25, dt);
      spinV = { yaw: spinV.yaw + (dAz / Math.max(dt, 1e-3) - spinV.yaw) * kv, pitch: spinV.pitch + (dEl / Math.max(dt, 1e-3) - spinV.pitch) * kv };
    } else if (spinV.yaw || spinV.pitch) {
      // momentum after release keeps turning around the last pivot
      const pv = lastPivot || p.target;
      p = orbitAround(p, pv, spinV.yaw * dt, spinV.pitch * dt);
      spinV = stepMomentum(spinV, dt, !!ctx.reducedMotion);
      moved = true;
    }

    // wheel and pinch: collect a log zoom and spend it over a few frames toward the cursor
    if (input.wheel) {
      zoomLog += input.wheel * (input.ctrlWheel ? 0.01 : 0.0012);
      zoomNdc = ndcOf(ctx, m.x, m.y);
      moved = true;
    }
    if (Math.abs(zoomLog) > 1e-4) {
      const part = zoomLog * damp(16, dt);
      zoomLog -= part;
      p = zoomToCursor(p, Math.exp(part), zoomNdc, fov, aspect);
    }
    if (input.dbl) {
      const h = hitGround(rayAt(ctx, input.dbl.x, input.dbl.y), 0);
      if (h) { touch(ctx); tween = flyToPoint(p, fov, h); }
    }

    if (moved) touch(ctx);
    if (tween) {
      const q = stepTween(tween, dt);
      p = { az: q.az, el: q.el, dist: q.dist, target: q.target };
      fov = q.fov;
      eff = p.dist;
      if (tween.done) tween = null;
    } else {
      const cxz = camXZ();
      p = clampOrbit(p, { ...limits(ctx), groundY: groundAt(ctx, cxz.x, cxz.z) });
      // the walkable box: target inside it, distance fitted to it, a low camera kept inside its walls
      p = boundPose(p, { bounds: ctx.garden && ctx.garden.bounds, fov, aspect, k: damp(20, dt) });
      if (c.spin && !moved && !drag) p.az += 0.12 * dt;
      eff = wallDistance(ctx, dt);
    }
    apply(ctx);
  };

  c.exit = (ctx) => {
    const cam = ctx.stage.camera;
    return { pos: [cam.position.x, cam.position.y, cam.position.z], target: [p.target.x, p.target.y, p.target.z] };
  };
  c.dispose = () => {};
  return c;
}
