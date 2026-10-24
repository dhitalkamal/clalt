// the page chrome of the garden campus: template panel on the left, camera bar at the bottom.
// it only builds buttons and reports clicks, the wiring lives in viewctl.js.
import { el, clear } from './dom.js';
import { CAMS } from './viewstate.js';

const BLURB = {
  hall: 'the original indoor hall',
  rows: 'cabins in neat rows along the back',
  village: 'pavilions in a ring around a green',
  river: 'cabins along the west bank of a river',
  street: 'one winding street to a fountain',
};

const HINT = 'drag pan, wheel zoom, WASD move, G ground, F follow, Esc top';

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
  const panel = document.getElementById('panel');
  const list = document.getElementById('templates');
  const bar = document.getElementById('bar');
  const camBtns = new Map();
  let tplBtns = new Map();
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

  const makeDefault = button('default', 'make default', () => h.makeDefault());
  makeDefault.title = 'show this template to everyone who opens the office';
  makeDefault.hidden = true;
  panel.append(makeDefault);

  return {
    // templates is the ordered list of {id, title}; key n picks the nth
    setTemplates(templates) {
      clear(list);
      tplBtns = new Map();
      templates.forEach((t, i) => {
        const b = el('button', 'tpl', el('span', 'k', String(i + 1)),
          el('span', 'txt', el('b', '', t.title || t.id), el('span', '', BLURB[t.id] || '')));
        b.type = 'button';
        b.addEventListener('click', () => h.template(t.id));
        b.setAttribute('aria-pressed', 'false');
        b.setAttribute('aria-keyshortcuts', String(i + 1));
        tplBtns.set(t.id, b);
        list.append(b);
      });
    },
    markTemplate(id) { for (const [k, b] of tplBtns) press(b, k === id); },
    markCam(id) { for (const [k, b] of camBtns) press(b, k === id); },
    setSpin(on) { press(spin, on); },
    setLabels(on) { press(labels, on); },
    // a reduced motion preference turns the spin off for good
    disableSpin(why) { spin.disabled = true; spin.title = why; },
    setEdit(on) { makeDefault.hidden = !on; },
    setBusy(on) { makeDefault.disabled = on; },
    // the camera bar and its hint show for every template. team lines and tags sit beside it
    showBar() {
      panel.hidden = false;
      bar.hidden = false;
      const hint = document.getElementById('hint-text');
      if (hint) hint.textContent = HINT;
    },
  };
}
