// keyboard shortcuts of the page as a pure function: a key event in, an action out
import { CAMS } from './viewstate.js';

const TEXT = new Set(['INPUT', 'TEXTAREA', 'SELECT']);

// templates is the ordered list, 1 picks the first. returns null for keys the page ignores.
export function actionFor(e, templates, { editing = false } = {}) {
  // a held key repeats keydown: toggles and presets fire once, movement is read from the held-keys set
  if (editing || e.repeat || e.ctrlKey || e.metaKey || e.altKey) return null;
  const t = e.target;
  if (t && (TEXT.has(t.tagName) || t.isContentEditable)) return null;
  const k = String(e.key || '').toLowerCase();
  if (/^[1-5]$/.test(k)) {
    const tpl = templates[Number(k) - 1];
    return tpl ? { type: 'template', id: tpl.id } : null;
  }
  const cam = CAMS.find((c) => c.key === k && /^[6-9]$/.test(k));
  if (cam) return { type: 'cam', id: cam.id };
  // movement keys (wasd, q e, r c, arrows) are read by the nav input state, not here
  const named = { g: 'ground', v: 'view', f: 'follow', p: 'spin', l: 'labels', home: 'home', escape: 'escape' };
  if (named[k]) return { type: named[k] };
  return null;
}
