// the sky controller: map-app camera over the island. wasd pan, left drag grabs the ground, right drag or q/e rotates,
// wheel zooms toward the cursor, r/pageup rise, c/pagedown lower, double click flies to a point.
import { poseFromLook } from '../garden/cameras.js';
import { panVector, clampPose, rayFromPose, hitGround, zoomToCursor, dirOf, MAX_H } from './skymath.js';
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
  let tween = null;
  let freed = false;
  const c = { name: 'sky', spin: false };

  const size = (ctx) => {
    const d = ctx.stage.renderer && ctx.stage.renderer.domElement;
    return { w: (d && d.clientWidth) || innerWidth, h: (d && d.clientHeight) || innerHeight };
  };
  const limits = (ctx) => ({ bounds: ctx.garden && ctx.garden.bounds, maxH });
  const gy = (ctx) => (ctx.garden && ctx.garden.groundY ? ctx.garden.groundY(p.target.x, p.target.z) : 0);

  function apply(ctx) {
    const { camera, controls } = ctx.stage;
    const d = dirOf(p.az, p.el);
    camera.position.set(p.target.x + d.x * p.dist, p.target.y + d.y * p.dist, p.target.z + d.z * p.dist);
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
    const cam = ctx.stage.camera;
    fov = cam.fov || 30;
    const ct = ctx.stage.controls && ctx.stage.controls.target;
    const f = from || { pos: [cam.position.x, cam.position.y, cam.position.z], target: ct ? [ct.x, ct.y, ct.z] : [0, 0, 0] };
    const q = poseFromLook({ x: f.pos[0], y: f.pos[1], z: f.pos[2] }, { x: f.target[0], y: f.target[1], z: f.target[2] }, fov);
    p = { az: q.az, el: q.el, dist: q.dist, target: { x: q.target.x, y: 0, z: q.target.z } };
    // coming from a low view (ground, follow): rise above the spot instead of staying at head height
    if (q.dist < 25) { p.el = Math.max(p.el, 0.9); p.dist = 35; p.target.x = f.pos[0]; p.target.z = f.pos[2]; }
    p = clampPose(p, { ...limits(ctx), maxH: Math.max(MAX_H, p.dist * Math.sin(p.el) * 1.1) });
    apply(ctx);
  };

  // fly to a preset pose {az, el, dist, target, fov}. a wide preset lifts the height cap so it is not clipped
  c.flyToPreset = (ctx, to, instant) => {
    maxH = Math.max(MAX_H, to.dist * Math.sin(to.el) * 1.15);
    freed = false;
    vel = { x: 0, z: 0 }; zoomLog = 0;
    if (instant) { p = { az: to.az, el: to.el, dist: to.dist, target: { ...to.target } }; fov = to.fov; tween = null; apply(ctx); return; }
    tween = flyToPose({ ...p, fov }, to);
  };

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
    if (!(m.buttons & 1) && Math.abs(vel.x) + Math.abs(vel.z) > 1e-3) { p.target.x += vel.x * dt; p.target.z += vel.z * dt; moved = true; }
    const yaw = (down(k, 'KeyE') ? 1 : 0) - (down(k, 'KeyQ') ? 1 : 0);
    if (yaw) { p.az += yaw * 1.3 * dt; moved = true; }
    const lift = (down(k, 'KeyR', 'PageUp') ? 1 : 0) - (down(k, 'KeyC', 'PageDown') ? 1 : 0);
    if (lift) { p.dist *= Math.exp(lift * 1.2 * dt); moved = true; }

    // left drag grabs the ground point under the cursor and keeps it there
    if (m.buttons & 1) {
      if (!grab) grab = hitGround(rayAt(ctx, m.x, m.y), 0) || false;
      else if (grab && (m.dx || m.dy)) {
        const h = hitGround(rayAt(ctx, m.x, m.y), 0);
        if (h) { p.target.x += grab.x - h.x; p.target.z += grab.z - h.z; vel = { x: (grab.x - h.x) / Math.max(dt, 1e-3) * 0.5, z: (grab.z - h.z) / Math.max(dt, 1e-3) * 0.5 }; moved = true; }
      }
    } else grab = null;
    // right drag rotates around the screen centre and tilts
    if ((m.buttons & 2) && (m.dx || m.dy)) { p.az -= m.dx * 0.005; p.el += m.dy * 0.004; moved = true; }

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
      if (tween.done) tween = null;
    } else {
      p = clampPose(p, { ...limits(ctx), groundY: gy(ctx) });
      if (c.spin && !moved && !grab) p.az += 0.12 * dt;
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
