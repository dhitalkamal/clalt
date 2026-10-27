// the page chrome of the garden campus: the camera bar at the bottom.
// it only builds buttons and reports clicks, the wiring lives in viewctl.js.
import { el } from './dom.js';
import { CAMS } from './viewstate.js';

const HINT = 'drag rotate, shift-drag move, wheel zoom, WASD move, G ground, Home or Esc overview';

function button(cls, label, onClick) {
  const b = el('button', cls, label);
  b.type = 'button';
  b.addEventListener('click', onClick);
  return b;
}

// a button with a key badge. the accessible name is the label alone, the key goes in aria-keyshortcuts.
function keyed(cls, label, key, onClick) {
  const b = button(cls, label, onClick);
  if (key) { b.append(el('kbd', '', key)); b.setAttribute('aria-keyshortcuts', key); }
  return b;
}

export function createChrome(h) {
  const bar = document.getElementById('bar');
  const camBtns = new Map();
  const press = (b, on) => { b.classList.toggle('on', on); b.setAttribute('aria-pressed', String(on)); };

  for (const c of CAMS) {
    const b = keyed('cam', c.label, c.key, () => h.cam(c.id));
    b.setAttribute('aria-pressed', 'false');
    camBtns.set(c.id, b);
    bar.append(b);
  }
  bar.append(el('div', 'gap'));
  const spin = keyed('', 'auto-spin', 'p', () => h.spin());
  const labels = keyed('', 'labels', 'l', () => h.labels());
  press(spin, false);
  press(labels, true);
  bar.append(spin, labels);

  return {
    markCam(id) { for (const [k, b] of camBtns) press(b, k === id); },
    setSpin(on) { press(spin, on); },
    setLabels(on) { press(labels, on); },
    // a reduced motion preference turns the spin off for good
    disableSpin(why) { spin.disabled = true; spin.title = why; },
    // the camera bar and its hint always show. team lines and tags sit beside it
    showBar() {
      bar.hidden = false;
      const hint = document.getElementById('hint-text');
      if (hint) hint.textContent = HINT;
    },
  };
}
