// the one place the page talks to the garden camera code (garden/cameras.js) and the navigation manager (nav/).
// both load on first use, so a missing or failing renderer cannot stop the page from booting.
// expected exports of garden/cameras.js: presetPose, setSpin, setSafe. nav/index.js exports createNav. garden and hall share it.
import { stage, app } from './app.js';
import { createNavHost } from './nav/host.js';

let mod = null;
let changeCb = () => {};
let host = null;
const reduced = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

async function cams() {
  if (!mod) mod = await import('./garden/cameras.js').catch((e) => { console.error(e); return {}; });
  return mod;
}

// the host builds one manager per template: its ctx reads the live layout, garden or hall (nav/host.js)
function getHost() {
  if (!host) {
    host = createNavHost({
      stage, app,
      load: async () => {
        const [{ createNav }, c, fx] = await Promise.all([import('./nav/index.js'), cams(), import('./garden/stagefx.js')]);
        return { createNav, presetPose: c.presetPose, fitFog: fx.fitFog };
      },
      reduced,
      aspect: () => innerWidth / Math.max(1, innerHeight),
    });
    host.onChange((s) => changeCb(s));
  }
  return host;
}
const getNav = () => getHost().get();

// a template switch: drop the manager so the next camera call builds one for the new layout
export function resetNav() { getHost().reset(); }

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
  getHost().setSafe(safe);
  // the hall presets read the host's safe fractions, so re-apply them (cameras.setSafe only covers the garden rig)
  const hall = app.layout && app.layoutJson && app.layoutJson.v !== 2 && app.layout.grids;
  if (hall && stage.nav) stage.nav.refit();
  const m = await cams();
  if (m.setSafe) m.setSafe(stage, safe);
}
