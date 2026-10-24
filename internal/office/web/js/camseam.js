// the one place the page talks to the garden camera code (garden/cameras.js) and the navigation manager (nav/).
// both load on first use, so a missing or failing renderer cannot stop the page from booting.
// expected exports of garden/cameras.js: presetPose, setSpin, setSafe. nav/index.js exports createNav.
import { stage, app } from './app.js';

let mod = null;
let navP = null;
let changeCb = () => {};
const reduced = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

async function cams() {
  if (!mod) mod = await import('./garden/cameras.js').catch((e) => { console.error(e); return {}; });
  return mod;
}

// the manager, made once. its ctx reads the live garden (a template switch rebuilds app.layout and stage.rig)
function getNav() {
  if (!navP) {
    navP = (async () => {
      const [{ createNav }, c, fx] = await Promise.all([import('./nav/index.js'), cams(), import('./garden/stagefx.js')]);
      const rig = () => stage.rig;
      const ctx = {
        stage, app,
        get reducedMotion() { return reduced(); },
        garden: {
          get walk() { return app.layout && app.layout.walk; },
          get bounds() { const b = rig() && rig().bb; return b ? { minX: b.x0, maxX: b.x1, minZ: b.z0, maxZ: b.z1 } : null; },
          groundY: () => 0,
        },
        presetPose: (id) => rig() && c.presetPose(id, { bb: rig().bb, aspect: rig().aspect(), safe: rig().safe, eye: rig().eye }),
        fitFog: (dist) => fx.fitFog(stage, dist),
        onChange: (s) => changeCb(s),
        onNotice: (m) => console.warn(m),
      };
      const nav = createNav(ctx);
      await nav.ready;
      stage.nav = nav;
      return nav;
    })().catch((e) => { console.error(e); return null; });
  }
  return navP;
}

// the page hears about mode and preset changes: cb({mode, preset})
export function onNavChange(cb) { changeCb = cb; }

// a preset id flies the sky camera, ground and sky switch the mode. sky with instant refits the top view for a new island
export async function goCam(id, instant) {
  const nav = await getNav();
  if (!nav) return;
  if (id === 'ground') nav.setMode('ground');
  else if (id === 'sky') { if (instant) nav.toPreset('top', { instant: true }); else nav.setMode('sky'); }
  else nav.toPreset(id, { instant });
}

// keyboard actions that belong to the camera: ground, view, follow, escape, home
export async function navAction(type) {
  const nav = await getNav();
  if (!nav) return;
  if (type === 'ground') nav.toggleGround();
  else if (type === 'view') nav.toggleView();
  else if (type === 'follow') nav.follow();
  else if (type === 'escape') nav.escape();
  else if (type === 'home') nav.toPreset('top');
}

export async function spinCam(on) {
  const nav = await getNav();
  if (nav) nav.setSpin(on);
}

// the screen fractions the camera keeps free for the page chrome
export async function safeCam(safe) {
  const m = await cams();
  if (m.setSafe) m.setSafe(stage, safe);
}
