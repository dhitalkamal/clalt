// what the page remembers about the view: the camera in the url hash. the office itself is the server's
// active layout, changed from the settings dialog. pure helpers, no browser needed.

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

// reads #cam=hero. anything that is not a known shape becomes null.
export function parseHash(hash) {
  const q = new URLSearchParams(String(hash || '').replace(/^#/, ''));
  const c = ALIAS[q.get('cam')] || q.get('cam');
  return { cam: CAMS.some((x) => x.id === c) ? c : null };
}

export function buildHash({ cam }) {
  return cam ? '#cam=' + cam : '';
}
