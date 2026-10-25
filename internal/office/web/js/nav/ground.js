// ground navigation: walk the garden like an agent, third or first person. no DOM listeners, the manager passes
// the input state to update. three.js is only needed by the avatar, which loads lazily (or is injected) so the
// node tests run without it.
import { stairsOf } from '../garden/walkgrid.js';
import { moveSlide } from './collide.js';
import { startJump, stepJump } from './jump.js';
import { pullDist, easeDist, camOffset } from './camfit.js';
import { forwardVec, clampPitch, moveIntent, stairFrame, stairDescOf, stairCoord, stairHeight, spawnPoint } from './groundmath.js';

const EYE = 0.9;
const NEAR = 0.9; // camera closer than this to the head hides the avatar
const HEADROOM = 0.95; // a jump under a deck stops this far below it
const HEAD = 0.8;
const TURN = 1.8; // radians per second on Q/E
const LOOK_X = 0.005;
const LOOK_Y = 0.004;
const BAND_PAD = 0.05; // the stair band is half the tread width plus this
const MOUTH = 0.6; // corridor past each end of the stair that stays open
const ENTRY = 0.5; // how far up the first steps count as the stair mouth

export function createGround(opts = {}) {
  let view = opts.view || 'third';
  const s = { x: 0, y: 0, z: 0, yaw: 0, pitch: 0, level: 0, stairs: false, base: 0, h: 0, vy: 0 };
  let walk = null;
  let fr = null;
  let band = 0.55;
  let top = 1;
  let avatar = null;
  let dead = false;
  let camSeen = false;
  let groundY = () => 0;
  let camRay = null;
  let back = 3.2; // follow distance and height, shorter in the hall
  let up = 1.6;
  let kc = 1; // eased share of the follow distance left after the pull-in
  let spaceHeld = false;
  let hidden = false; // avatar hidden because the camera is too close
  let sm = null; // smoothed camera spot before the pull-in

  // walkable under the current mode. the stair is its own region between the ground and the deck
  const free = (x, z) => {
    if (!fr) return walk.walkable(x, z, s.level);
    const { s: a, lat } = stairCoord(fr, x, z);
    const inBand = Math.abs(lat) <= band;
    if (s.stairs) {
      if (a >= 0 && a <= fr.len) return inBand;
      if (inBand && a >= -MOUTH && a <= fr.len + MOUTH) return true;
      return walk.walkable(x, z, a < 0 ? 0 : top);
    }
    if (walk.walkable(x, z, s.level)) return true;
    // the mouths count as walkable in the stair's own corridor so the disc fits the narrow ends
    return s.level === 0 ? inBand && a >= -MOUTH && a <= ENTRY : inBand && a <= fr.len + MOUTH && a >= fr.len - ENTRY;
  };

  // after a move: step onto or off the stair and set the floor height (base) and y = base + jump height
  function settle() {
    floor();
    s.y = s.base + s.h;
  }

  function floor() {
    const gy = groundY(s.x, s.z);
    if (fr) {
      const { s: a, lat } = stairCoord(fr, s.x, s.z);
      const inBand = Math.abs(lat) <= band;
      if (s.stairs) {
        if (a < 0) { s.stairs = false; s.level = 0; } else if (a > fr.len) { s.stairs = false; s.level = top; }
      } else if (inBand && a >= 0 && a < fr.len) s.stairs = true; // the stair frame decides, not the grid (the landing overlaps the top steps)
      if (s.stairs) { s.base = stairHeight(fr, a); s.h = 0; s.vy = 0; return; }
    }
    s.base = s.level === top ? (fr ? fr.rise : 1.29) : gy;
  }

  // a deck overhead (level 0 under the deck) caps the jump so the head stays below it
  const lid = () => (s.level === 0 && !s.stairs && walk.walkable(s.x, s.z, top) ? (fr ? fr.rise : 1.29) - HEADROOM - s.base : Infinity);

  const c = {
    name: 'ground',
    state: s,
    enter(ctx, from, o = {}) {
      dead = false;
      camSeen = false;
      kc = 1;
      hidden = false;
      if (o.view) view = o.view;
      walk = ctx.garden.walk;
      groundY = ctx.garden.groundY || (() => 0);
      // the walk brings its own stairs descriptor (the hall), else it is read off the garden site
      const gst = walk.site ? stairsOf(walk.site) : null;
      const sd = walk.stairDesc !== undefined ? walk.stairDesc : gst && stairDescOf(gst);
      fr = sd ? stairFrame(sd) : null;
      band = fr ? fr.width / 2 + BAND_PAD : 0.55;
      top = fr ? fr.top : 1;
      const hall = walk.stairDesc !== undefined;
      back = hall ? 3.0 : 3.2;
      up = hall ? 2.0 : 1.6;
      const ground = (x, z) => walk.walkable(x, z, 0);
      let wx = NaN;
      let wz = NaN;
      let yaw = 0;
      if (from) {
        const [px, py, pz] = from.pos;
        const [tx, ty, tz] = from.target;
        const hx = tx - px;
        const hz = tz - pz;
        if (Math.hypot(hx, hz) > 1e-3) yaw = Math.atan2(hx, hz);
        // the ground point under the screen centre: where the view ray meets y = 0
        if (ty < py) { const t = py / (py - ty); wx = px + (tx - px) * t; wz = pz + (tz - pz) * t; }
      }
      const avoid = ctx.garden.blockers ? ctx.garden.blockers() : [];
      const at = spawnPoint(ground, wx, wz, { avoid, fallback: { x: walk.spawn.ap.x, z: walk.spawn.ap.z }, yaw });
      Object.assign(s, { x: at.x, z: at.z, yaw: at.yaw, pitch: 0, level: 0, stairs: false, h: 0, vy: 0 });
      settle();
      this.setView(view);
      const make = opts.makeAvatar || ((cx) => import('./avatar.js').then((m) => m.createAvatar(cx)));
      camRay = opts.camRay || null;
      if (!camRay) import('./camray.js').then((m) => { if (!dead && !camRay) camRay = m.createCamRay(ctx); }).catch(() => {});
      Promise.resolve(make(ctx)).then((a) => {
        if (dead || avatar) { a.dispose(); return; }
        avatar = a;
        avatar.setVisible(view === 'third');
      }).catch(() => {});
    },
    update(ctx, dt, input) {
      dt = Math.min(dt, 0.1);
      const k = input.keys;
      s.yaw += ((k.has('KeyQ') ? 1 : 0) - (k.has('KeyE') ? 1 : 0)) * TURN * dt;
      if (input.mouse.buttons & 1) {
        s.yaw -= input.mouse.dx * LOOK_X;
        s.pitch = clampPitch(s.pitch - input.mouse.dy * LOOK_Y);
      }
      const v = moveIntent(k, s.yaw);
      let speed = 0;
      if (v.moving) {
        const p = moveSlide(s, v.vx * dt, v.vz * dt, free);
        speed = Math.hypot(p.x - s.x, p.z - s.z) / dt;
        s.x = p.x;
        s.z = p.z;
        floor();
      }
      // one jump per press: a held key or auto-repeat does not re-jump on landing
      const space = k.has('Space');
      if (space && !spaceHeld) startJump(s, s.stairs);
      spaceHeld = space;
      stepJump(s, dt, lid());
      s.y = s.base + s.h;
      const cam = ctx.stage.camera;
      const [fx, fz] = forwardVec(s.yaw);
      const cp = Math.cos(s.pitch);
      // the camera follows the floor, plus half the jump (none in reduced motion) so a hop does not shake the view
      const cy = s.base + (ctx.reducedMotion ? 0 : s.h * 0.5);
      if (view === 'first') {
        cam.position.set(s.x, cy + EYE, s.z);
        cam.lookAt(s.x + fx * cp * 5, cy + EYE + Math.sin(s.pitch) * 5, s.z + fz * cp * 5);
      } else {
        const want = [s.x - fx * back * cp, Math.max(cy + 0.3, cy + up - Math.sin(s.pitch) * back), s.z - fz * back * cp];
        const a = ctx.reducedMotion || !camSeen ? 1 : 1 - Math.exp(-dt * 10);
        if (!sm || !camSeen) sm = want.slice();
        camSeen = true;
        for (let i = 0; i < 3; i++) sm[i] += (want[i] - sm[i]) * a;
        const head = [s.x, cy + HEAD, s.z];
        // a pulled-in camera sits higher (from last frame's pull), then one ray head to camera decides the pull
        const roof = camRay && camRay.above ? camRay.above(head, 2) : null; // a deck overhead caps the camera height
        const topY = roof == null ? Infinity : head[1] + Math.max(0.2, roof - 0.15);
        const raised = [sm[0], Math.min(topY, sm[1] + camOffset(kc * back, back).up - 0.8), sm[2]];
        const len = Math.hypot(raised[0] - head[0], raised[1] - head[1], raised[2] - head[2]) || 1;
        const hit = camRay ? camRay.hit(head, raised) : null;
        kc = easeDist(kc, pullDist(len, hit) / len, dt, ctx.reducedMotion);
        cam.position.set(head[0] + (raised[0] - head[0]) * kc, head[1] + (raised[1] - head[1]) * kc, head[2] + (raised[2] - head[2]) * kc);
        // close to the head the view turns ahead (along the heading) instead of down at the avatar
        const w = Math.min(1, Math.max(0, (1.3 - kc * len) / 1.0));
        cam.lookAt(s.x + fx * 4 * w, cy + HEAD, s.z + fz * 4 * w);
        // the camera is inside the character: hide it so the view is not a wall of back
        const near = kc * len < NEAR;
        if (near !== hidden) { hidden = near; avatar?.setVisible(!hidden); }
      }
      avatar?.update({ x: s.x, y: s.y, z: s.z, yaw: s.yaw, moving: v.moving, running: v.running, speed, air: s.h > 0, vy: s.vy }, dt);
    },
    exit(ctx) {
      const p = ctx.stage.camera.position;
      const [fx, fz] = forwardVec(s.yaw);
      const cp = Math.cos(s.pitch);
      const eye = view === 'first' ? [s.x, s.y + EYE, s.z] : [p.x, p.y, p.z];
      // the target looks along the walking heading from the eye, a few units out, on the ground side
      return { pos: eye, target: [eye[0] + fx * cp * 6, Math.max(0, eye[1] + Math.sin(s.pitch) * 6), eye[2] + fz * cp * 6] };
    },
    setView(v) {
      view = v === 'first' ? 'first' : 'third';
      camSeen = false;
      hidden = false;
      avatar?.setVisible(view === 'third');
    },
    dispose() {
      dead = true;
      avatar?.dispose();
      avatar = null;
    },
  };
  return c;
}
