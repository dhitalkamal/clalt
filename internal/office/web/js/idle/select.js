// one labelled select of idle choices, shared by the settings page and the quick menu
import { el } from '../dom.js';
import { optionsFor } from './view.js';

let serial = 0;

// h: {value, same, rooms, label (accessible name)}. same is the name the empty option falls back to, null on the default rows
export function choiceSelect(h) {
  const sel = el('select');
  sel.id = 'idle-select-' + ++serial;
  sel.setAttribute('aria-label', h.label);
  for (const o of optionsFor(h.rooms, h.same, h.value)) {
    const opt = el('option', '', o.text);
    opt.value = o.value;
    sel.append(opt);
  }
  sel.value = h.value || '';
  return sel;
}
