// what the page remembers about the view: the camera in the url hash. the office itself is the server's
// active layout, changed from the settings dialog. pure helpers, no browser needed.

// the bar offers two navigation modes: ground (g) and the free sky camera
export const CAMS = [
  { id: 'ground', label: 'ground', key: 'g' },
  { id: 'sky', label: 'sky', key: '' },
];
// camera ids of older urls: the fixed presets now open the sky camera at the overview
const ALIAS = { eye: 'ground', orbit: 'sky', top: 'sky', plan: 'sky', iso: 'sky', hero: 'sky' };

// what the bar shows for a nav state: the overview framing is still the sky camera
export const camOfNav = (s) => (s && s.mode === 'ground' ? 'ground' : 'sky');

// the camera a new layout starts in: ground would drop the walker into a layout it has not seen
export const camAfterSwitch = () => 'sky';

// reads #cam=ground. anything that is not a known shape becomes null.
export function parseHash(hash) {
  const q = new URLSearchParams(String(hash || '').replace(/^#/, ''));
  const c = ALIAS[q.get('cam')] || q.get('cam');
  return { cam: CAMS.some((x) => x.id === c) ? c : null };
}

export function buildHash({ cam }) {
  return cam ? '#cam=' + cam : '';
}
