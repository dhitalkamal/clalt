// small helpers for the field notebook look: state stamps, account chips, review flags.
// the pure parts return class and text, the node builders wrap them. state is always text too.
import { el } from './dom.js';

const WORDS = { busy: 'working', idle: 'idle', 'needs-you': 'needs you' };
const CLASSES = { busy: 's-busy', 'needs-you': 's-need' };

// {cls, text} for a state. a given text wins, only the needs-you spelling is cleaned up.
export function stampInfo(state, text) {
  const cls = 'stamp ' + (CLASSES[state] || 's-idle');
  const t = text === undefined || text === null || text === '' ? WORDS[state] || 'idle' : String(text);
  return { cls, text: t.replace(/needs-you/g, 'needs you') };
}

export function stampNode(state, text) {
  const s = stampInfo(state, text);
  return el('span', s.cls, el('i'), s.text);
}

const TINTS = ['a-work', 'a-personal', 'a-client'];

// tape label tint by account name: the usual names keep their colour, others hash into the three
export function chipClass(label) {
  const name = String(label || '').trim().toLowerCase();
  const known = TINTS.find((t) => t === 'a-' + name);
  if (known) return 'chip ' + known;
  let h = 0;
  for (const ch of name) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  return 'chip ' + (name ? TINTS[h % TINTS.length] : TINTS[0]);
}

export const chipNode = (label) => el('span', chipClass(label), String(label || '-'));

// a stamp for the review tones that are not a severity
export function toneFlag(tone) {
  return tone === 'error' || tone === 'clean' || tone === 'skipped' ? { cls: 'flag ' + tone, text: tone } : null;
}
