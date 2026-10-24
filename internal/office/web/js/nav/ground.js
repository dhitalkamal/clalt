// ground navigation: walk the garden like an agent, third or first person. no DOM listeners, the manager passes
// the input state to update. three.js is only needed by the avatar, which loads lazily (or is injected) so the
// node tests run without it.
import { stairsOf } from '../garden/walkgrid.js';
import { moveSlide } from './collide.js';
import { forwardVec, clampPitch, moveIntent, stairFrame, stairDescOf, stairCoord, stairHeight, spawnPoint } from './groundmath.js';

const EYE = 0.9;
const BACK = 3.2;
const UP = 1.6;
const HEAD = 0.8;
const TURN = 1.8; // radians per second on Q/E
const LOOK_X = 0.005;
const LOOK_Y = 0.004;
const BAND_PAD = 0.05; // the stair band is half the tread width plus this
const MOUTH = 0.6; // corridor past each end of the stair that stays open
const ENTRY = 0.5; // how far up the first steps count as the stair mouth

export function createGround(opts = {}) {
  let view = opts.view || 'third';
  const s = { x: 0, y: 0, z: 0, yaw: 0, pitch: 0, level: 0, stairs: false };
  let walk = null;
  let fr = null;
  let band = 0.55;
  let top = 1;
  let avatar = null;
  let dead = false;
  let camSeen = false;
  let groundY = () => 0;

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

  // after a move: step onto or off the stair and set the height
  function settle() {
    const gy = groundY(s.x, s.z);
    if (fr) {
      const { s: a, lat } = stairCoord(fr, s.x, s.z);
      const inBand = Math.abs(lat) <= band;
      if (s.stairs) {
        if (a < 0) { s.stairs = false; s.level = 0; } else if (a > fr.len) { s.stairs = false; s.level = top; }
      } else if (inBand && a >= 0 && a < fr.len) s.stairs = true; // the stair frame decides, not the grid (the landing overlaps the top steps)
      if (s.stairs) { s.y = stairHeight(fr, a); return; }
    }
    s.y = s.level === top ? (fr ? fr.rise : 1.29) : gy;
  }

  const c = {
    name: 'ground',
    state: s,
    enter(ctx, from, o = {}) {
      dead = false;
      camSeen = false;
      if (o.view) view = o.view;
      walk = ctx.garden.walk;
      groundY = ctx.garden.groundY || (() => 0);
      // the walk brings its own stairs descriptor (the hall), else it is read off the garden site
      const gst = walk.site ? stairsOf(walk.site) : null;
      const sd = walk.stairDesc !== undefined ? walk.stairDesc : gst && stairDescOf(gst);
      fr = sd ? stairFrame(sd) : null;
      band = fr ? fr.width / 2 + BAND_PAD : 0.55;
      top = fr ? fr.top : 1;
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
      Object.assign(s, { x: at.x, z: at.z, yaw: at.yaw, pitch: 0, level: 0, stairs: false });
      settle();
      this.setView(view);
      const make = opts.makeAvatar || ((cx) => import('./avatar.js').then((m) => m.createAvatar(cx)));
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
      if (v.moving) {
        const p = moveSlide(s, v.vx * dt, v.vz * dt, free);
        s.x = p.x;
        s.z = p.z;
        settle();
      }
      const cam = ctx.stage.camera;
      const [fx, fz] = forwardVec(s.yaw);
      const cp = Math.cos(s.pitch);
      if (view === 'first') {
        cam.position.set(s.x, s.y + EYE, s.z);
        cam.lookAt(s.x + fx * cp * 5, s.y + EYE + Math.sin(s.pitch) * 5, s.z + fz * cp * 5);
      } else {
        const want = [s.x - fx * BACK * cp, Math.max(s.y + 0.3, s.y + UP - Math.sin(s.pitch) * BACK), s.z - fz * BACK * cp];
        const p = cam.position;
        const a = ctx.reducedMotion || !camSeen ? 1 : 1 - Math.exp(-dt * 10);
        camSeen = true;
        p.set(p.x + (want[0] - p.x) * a, p.y + (want[1] - p.y) * a, p.z + (want[2] - p.z) * a);
        cam.lookAt(s.x, s.y + HEAD, s.z);
      }
      avatar?.update({ x: s.x, y: s.y, z: s.z, yaw: s.yaw, moving: v.moving, running: v.running }, dt);
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
