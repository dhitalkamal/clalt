// owns the navigation manager for the page: builds it on first use, hands it the ctx of whichever template is on
// screen (garden or hall) and throws it away when the template changes so no listener or avatar outlives it.
// deps: {stage, app, load() -> {createNav, presetPose, fitFog}, reduced(), aspect(), navOpts?}. no DOM, no three.js.
import { hallCtx } from './hallctx.js';
import { blockers } from './blockers.js';

export function createNavHost(deps) {
  const { stage, app } = deps;
  let navP = null;
  let gen = 0;
  let safe = {};
  let changeCb = () => {};

  // the hall when the live layout is a v1 one, else the garden
  const hall = () => (app.layout && app.layoutJson && app.layoutJson.v !== 2 && app.layout.grids ? hallCtx(app.layout) : null);
  const rig = () => stage.rig;

  function makeCtx(mods) {
    return {
      stage, app,
      get reducedMotion() { return deps.reduced(); },
      garden: {
        get walk() { const h = hall(); return h ? h.walk : app.layout && app.layout.walk; },
        get bounds() {
          const h = hall();
          if (h) return h.bounds;
          const b = rig() && rig().bb;
          return b ? { minX: b.x0, maxX: b.x1, minZ: b.z0, maxZ: b.z1 } : null;
        },
        // agents and seats the ground spawn keeps clear of
        blockers: () => blockers(app, hall()),
        groundY: (x, z, level) => { const h = hall(); return h ? h.groundY(x, z, level) : 0; },
      },
      presetPose: (id) => {
        const h = hall();
        if (h) return mods.presetPose(id, { bb: h.bb, aspect: deps.aspect(), safe, eye: null });
        const r = rig();
        return r && mods.presetPose(id, { bb: r.bb, aspect: r.aspect(), safe: r.safe, eye: r.eye });
      },
      fitFog: (dist) => {
        mods.fitFog(stage, dist);
        // no ground or haze behind the hall: keep the whole box clear so its edges stay crisp
        if (hall() && stage.scene && stage.scene.fog) { stage.scene.fog.near = dist * 3; stage.scene.fog.far = dist * 9; }
      },
      onChange: (s) => changeCb(s),
      onNotice: (m) => console.warn(m),
    };
  }

  return {
    // the manager, made once per template. resolves to null when it cannot load
    get() {
      if (!navP) {
        const mine = ++gen;
        navP = (async () => {
          const mods = await deps.load();
          const nav = mods.createNav(makeCtx(mods), deps.navOpts);
          try { await nav.ready; } catch (e) { nav.dispose(); throw e; }
          // a reset while loading: this manager belongs to a template that is gone
          if (mine !== gen) { nav.dispose(); return null; }
          stage.nav = nav;
          stage.navFailed = false;
          return nav;
        })().catch((e) => {
          console.warn('camera navigation failed to load', e);
          // forget the failure so the next get retries, and let the page fall back to its own orbit
          if (mine === gen) { navP = null; stage.navFailed = true; }
          return null;
        });
      }
      return navP;
    },
    // drop the manager (a template switch). the next get builds a fresh one for the new layout
    reset() {
      gen++;
      const p = navP;
      navP = null;
      stage.navFailed = false;
      const old = stage.nav;
      stage.nav = null;
      if (old) old.dispose();
      return p;
    },
    onChange(cb) { changeCb = cb; },
    // screen fractions the hall presets keep free (the garden keeps its own in stage.rig.safe)
    setSafe(s) { safe = s; },
    get safe() { return safe; },
  };
}
