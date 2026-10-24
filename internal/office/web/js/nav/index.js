// the navigation mode manager: one controller drives the camera at a time (sky, ground or follow).
// a controller is {name, enter(ctx, fromPose), update(ctx, dt, input), exit(ctx) -> pose, dispose()}, a pose is
// {pos:[x,y,z], target:[x,y,z]}. update may return 'back' (or set done) to hand control back to the previous mode.
import { createInput } from './input.js';

export const TWEEN_SECONDS = 0.6;
const ease = (t) => t * t * (3 - 2 * t);
const lerp = (a, b, k) => a + (b - a) * k;

// a missing module resolves to null, the manager then stays in sky. literal paths so the page module check sees them
const tryLoad = async (load, fn, arg) => {
  try { return (await load())[fn](arg); } catch (e) { return null; }
};
const defaultFactories = () => ({
  sky: async () => (await import('./sky.js')).createSky(),
  ground: (o) => tryLoad(() => import('./ground.js'), 'createGround', o),
  follow: (id) => tryLoad(() => import('./follow.js'), 'createFollow', id),
});

// camera position plus a point ahead of it, read from the camera quaternion
function readPose(cam, controls) {
  const p = [cam.position.x, cam.position.y, cam.position.z];
  const q = cam.quaternion;
  if (q) {
    const fx = -2 * (q.x * q.z + q.w * q.y); const fy = -2 * (q.y * q.z - q.w * q.x); const fz = -(1 - 2 * (q.x * q.x + q.y * q.y));
    return { pos: p, target: [p[0] + fx * 10, p[1] + fy * 10, p[2] + fz * 10] };
  }
  const t = controls && controls.target;
  return { pos: p, target: t ? [t.x, t.y, t.z] : [p[0], p[1], p[2] - 10] };
}

export function createNav(ctx, opts = {}) {
  const f = opts.factories || defaultFactories();
  const cam = ctx.stage.camera;
  const dom = ctx.stage.renderer && ctx.stage.renderer.domElement;
  const input = opts.input || (dom && typeof window !== 'undefined' ? createInput(dom)
    : { keys: new Set(), mouse: { x: 0, y: 0, dx: 0, dy: 0, buttons: 0, over: false }, wheel: 0, dbl: null, consume() {}, dispose() {} });
  const st = { mode: 'sky', ctrl: null, prev: 'sky', tw: null, spin: false, view: 'third', preset: null, busy: false };

  const emit = () => { if (ctx.onChange) ctx.onChange({ mode: st.mode, preset: st.preset }); };
  const userFree = ctx.onFree;
  ctx.onFree = () => { st.preset = null; emit(); if (userFree) userFree(); };

  async function make(name, arg) {
    try { return f[name] ? await f[name](arg) : null; } catch (e) { console.error(e); return null; }
  }

  async function switchTo(name, next, from, o) {
    const old = st.ctrl;
    const fromPose = old ? old.exit(ctx) : readPose(cam, ctx.stage.controls);
    const fromFov = cam.fov;
    if (old) old.dispose();
    if (name === 'sky') next.spin = st.spin;
    st.prev = from;
    st.ctrl = next;
    st.mode = name;
    next.enter(ctx, old ? fromPose : null, o);
    st.tw = old && !ctx.reducedMotion ? { t: 0, from: fromPose, fromFov } : null;
    if (name !== 'sky') st.preset = null;
    emit();
    return true;
  }

  const api = {
    get mode() { return st.mode; },
    get tweening() { return !!st.tw; },
    get spin() { return st.spin; },
    get view() { return st.view; },
    ready: null,

    // switch controller. resolves true on success, false when the module is missing (the mode stays as it was)
    async setMode(name, o = {}) {
      await api.ready;
      if (name === 'follow') return api.follow(o.agentId);
      if (name === st.mode && !o.view) return true;
      if (name === st.mode && st.ctrl.setView) { st.ctrl.setView(o.view); return true; }
      const next = await make(name, o);
      if (!next) { if (ctx.onNotice) ctx.onNotice(name + ' mode is not available'); return false; }
      return switchTo(name, next, st.mode === 'follow' ? 'sky' : st.mode);
    },

    async follow(agentId) {
      await api.ready;
      const a = ctx.app || {};
      const id = agentId || (a.selected && a.selected.s && a.selected.s.id) || (a.hover && a.hover.s && a.hover.s.id);
      if (!id) return false;
      const next = await make('follow', id);
      if (!next) { if (ctx.onNotice) ctx.onNotice('follow mode is not available'); return false; }
      return switchTo('follow', next, st.mode === 'follow' ? st.prev : st.mode);
    },

    // fly the sky camera to a preset pose supplied by ctx.presetPose(id)
    async toPreset(id, o = {}) {
      await api.ready;
      if (st.mode !== 'sky' && !(await api.setMode('sky'))) return false;
      const to = ctx.presetPose && ctx.presetPose(id);
      if (!to) return false;
      st.preset = id;
      st.ctrl.flyToPreset(ctx, to, !!o.instant || !!ctx.reducedMotion);
      emit();
      return true;
    },

    setSpin(on) { st.spin = !!on; if (st.mode === 'sky' && st.ctrl) st.ctrl.spin = st.spin; },
    toggleGround() { return api.setMode(st.mode === 'ground' ? 'sky' : 'ground'); },
    toggleView() { st.view = st.view === 'first' ? 'third' : 'first'; return api.setMode('ground', { view: st.view }); },
    // Esc: ground and follow go back to sky where they are, sky goes to the top preset
    escape() { return st.mode === 'sky' ? api.toPreset('top') : api.setMode('sky'); },

    update(dt) {
      const c = st.ctrl;
      if (!c) { input.consume(); return; }
      let r;
      try { r = c.update(ctx, dt, input); } catch (e) { console.error(e); r = 'back'; }
      input.consume();
      if (st.tw) {
        st.tw.t += dt / TWEEN_SECONDS;
        const k = ease(Math.min(1, st.tw.t));
        const to = readPose(cam, ctx.stage.controls);
        const a = st.tw.from;
        const pos = [0, 1, 2].map((i) => lerp(a.pos[i], to.pos[i], k));
        const tg = [0, 1, 2].map((i) => lerp(a.target[i], to.target[i], k));
        cam.position.set(pos[0], pos[1], pos[2]);
        cam.lookAt(tg[0], tg[1], tg[2]);
        if (st.tw.t >= 1) st.tw = null;
      }
      if ((r === 'back' || c.done) && !st.busy) {
        st.busy = true;
        api.setMode(st.mode === 'follow' ? st.prev : 'sky').finally(() => { st.busy = false; });
      }
    },

    dispose() { if (st.ctrl) st.ctrl.dispose(); st.ctrl = null; input.dispose(); },
  };
  ctx.nav = api;

  api.ready = (async () => {
    const sky = await make('sky');
    if (!sky) return;
    st.ctrl = sky;
    sky.spin = st.spin;
    sky.enter(ctx, null);
  })();
  return api;
}
