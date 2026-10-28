// the day view as dom, from the model of model.js. everything is set as text.
import { el } from '../dom.js';
import { chipNode } from '../notebook.js';
import { limitTone, meterPct } from './format.js';

function factsRow(facts) {
  const dl = el('dl', 'facts');
  for (const [k, v] of facts) dl.append(el('div', '', el('dt', '', k), el('dd', '', v)));
  return dl;
}

// the 5h percent of the day: the text, and a paper-framed meter of the latest reading with a tick at the peak
function limitRow(a) {
  const row = el('div', 'limit', el('span', 'limtext', a.limit));
  const v = meterPct(a.now5h == null ? a.peak5h : a.now5h);
  const m = el('div', 'limmeter tone-' + limitTone(v), el('i', 'limfill'));
  m.setAttribute('role', 'meter');
  m.setAttribute('aria-valuemin', '0');
  m.setAttribute('aria-valuemax', '100');
  m.setAttribute('aria-valuenow', String(v));
  m.setAttribute('aria-valuetext', v + ' percent');
  m.setAttribute('aria-label', 'latest 5h limit reading');
  m.firstChild.style.width = v + '%';
  if (a.now5h != null && a.peak5h != null) {
    const tick = el('b', 'limpeak');
    tick.style.left = meterPct(a.peak5h) + '%';
    m.append(tick);
  }
  row.append(m);
  return row;
}

function detail(label, text) {
  return el('div', 'detail', el('dt', '', label), el('dd', '', text));
}

function project(p) {
  const box = el('div', 'proj', el('h4', '', p.title), factsRow(p.facts));
  const dl = el('dl', 'details');
  if (p.subagents) dl.append(detail('Subagents', p.subagents));
  if (p.tools) dl.append(detail('Tools', p.tools));
  if (p.files) dl.append(detail('Files', p.files));
  if (p.failureCount) {
    const d = detail('Failures', String(p.failureCount));
    d.classList.add('bad');
    const ul = el('ul', 'fails');
    for (const f of p.failures) ul.append(el('li', '', f));
    d.querySelector('dd').append(ul);
    dl.append(d);
  }
  if (dl.childNodes.length) box.append(dl);
  return box;
}

// m is summaryModel(reply)
export function renderDay(m) {
  const root = el('div', 'dayview');
  root.append(el('section', 'total', el('h3', '', 'All accounts'), factsRow(m.headline)));
  if (m.truncated) root.append(el('p', 'warn', 'Some transcripts were not read (size limit), so the numbers may be low.'));
  if (m.empty) {
    root.append(el('p', 'quiet', 'Nothing happened on this day.'));
    return root;
  }
  for (const a of m.accounts) {
    const sec = el('section', 'acct', el('h3', '', chipNode(a.title)), factsRow(a.facts));
    if (a.limit) sec.append(limitRow(a));
    for (const p of a.projects) sec.append(project(p));
    root.append(sec);
  }
  return root;
}
