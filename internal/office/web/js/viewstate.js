// what the page remembers about the view: template and camera in the url hash, the last template in
// localStorage. pure helpers, storage is passed in so tests need no browser.

// presets sit on 6 to 9, ground and sky are navigation modes (ground has g, sky is the default free camera)
export const CAMS = [
  { id: 'top', label: 'top', key: '6' },
  { id: 'plan', label: 'plan', key: '7' },
  { id: 'iso', label: 'iso', key: '8' },
  { id: 'hero', label: 'hero', key: '9' },
  { id: 'ground', label: 'ground', key: 'g' },
  { id: 'sky', label: 'sky', key: '' },
];
// camera ids of older urls
const ALIAS = { eye: 'ground', orbit: 'sky' };

// the camera a new layout starts in: ground would drop the walker into a layout it has not seen
export const camAfterSwitch = (cam) => (cam === 'ground' ? 'sky' : cam);

const ORDER = ['hall', 'rows', 'village', 'river', 'street'];
const STORE_KEY = 'clalt-office-template';
const ID = /^[a-z0-9-]{1,40}$/;

// reads #template=river&cam=hero. anything that is not a known shape becomes null.
export function parseHash(hash) {
  const q = new URLSearchParams(String(hash || '').replace(/^#/, ''));
  const t = q.get('template');
  const c = ALIAS[q.get('cam')] || q.get('cam');
  return { template: t && ID.test(t) ? t : null, cam: CAMS.some((x) => x.id === c) ? c : null };
}

export function buildHash({ template, cam }) {
  const parts = [];
  if (template) parts.push('template=' + template);
  if (cam) parts.push('cam=' + cam);
  return parts.length ? '#' + parts.join('&') : '';
}

// storage access can throw (private mode, blocked site data), so every call is wrapped
export function remember(storage, id) {
  try { storage.setItem(STORE_KEY, id); } catch (e) { /* not remembered, fine */ }
}

export function recall(storage) {
  try { const v = storage.getItem(STORE_KEY); return v && ID.test(v) ? v : null; } catch (e) { return null; }
}

// the template to start on: the hash, then the last pick, else null so the server default shows.
// templates is the known list, or null before the first answer (a well formed id is trusted then).
export function pickTemplate({ hash, stored, templates }) {
  const known = (id) => id && (!templates || templates.some((t) => t.id === id));
  if (known(hash)) return hash;
  if (known(stored)) return stored;
  return null;
}

// hall first, then the garden templates in the order of the plan, then anything else the server lists
export function orderTemplates(list) {
  const rank = (id) => { const i = ORDER.indexOf(id); return i < 0 ? ORDER.length : i; };
  const out = [...list];
  if (!out.some((t) => t.id === 'hall')) out.push({ id: 'hall', title: 'Hall' });
  return out.sort((a, b) => rank(a.id) - rank(b.id));
}
