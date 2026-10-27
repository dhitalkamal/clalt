// draws one history chart as svg from chart.js numbers. keyboard: arrows, Home and End move
// between days and the line under the chart reads the day aloud. no library, no canvas.
import { chartModel, readout } from './chart.js';

const NS = 'http://www.w3.org/2000/svg';
const HEIGHT = 170;

function svgEl(tag, attrs, text) {
  const n = document.createElementNS(NS, tag);
  for (const [k, v] of Object.entries(attrs || {})) n.setAttribute(k, String(v));
  if (text != null) n.textContent = text;
  return n;
}

// opts: { title, who, days, key, cls, width }
export function drawChart(opts) {
  const { title, who, days, key, cls, width } = opts;
  const m = chartModel({ days, key, width, height: HEIGHT });
  const fig = document.createElement('figure');
  fig.className = 'chart';
  const cap = document.createElement('figcaption');
  cap.textContent = title;
  const svg = svgEl('svg', { viewBox: `0 0 ${width} ${HEIGHT}`, width, height: HEIGHT, class: 'plot ' + cls, tabindex: 0, role: 'group',
    'aria-label': `${title}, ${who}. Left and right arrow keys read one day at a time.` });
  const p = m.plot;
  for (const t of m.yTicks) {
    svg.append(svgEl('line', { x1: p.x, x2: p.x + p.w, y1: t.y, y2: t.y, class: 'grid' }));
    svg.append(svgEl('text', { x: p.x - 5, y: t.y + 3, class: 'ylab', 'text-anchor': 'end' }, t.label));
  }
  const rects = m.bars.map((b) => {
    const r = svgEl('rect', { x: b.x, y: b.y, width: b.w, height: b.h, class: 'col' });
    svg.append(r);
    return r;
  });
  for (const t of m.xTicks) svg.append(svgEl('text', { x: t.x, y: p.y + p.h + 14, class: 'xlab', 'text-anchor': 'middle' }, t.text));
  // a slot per day that takes the pointer, wider than a thin bar
  const slot = p.w / Math.max(1, days.length);
  const out = document.createElement('p');
  out.className = 'readout';
  out.setAttribute('aria-live', 'polite');
  let cur = days.length - 1;
  const show = (i) => {
    if (!days.length) return;
    cur = Math.max(0, Math.min(days.length - 1, i));
    rects.forEach((r, k) => r.classList.toggle('cur', k === cur));
    out.textContent = readout(days[cur]);
  };
  days.forEach((_, i) => {
    const hit = svgEl('rect', { x: p.x + i * slot, y: p.y, width: slot, height: p.h, class: 'hit' });
    hit.addEventListener('pointerenter', () => show(i));
    hit.addEventListener('click', () => show(i));
    svg.append(hit);
  });
  svg.addEventListener('keydown', (e) => {
    const next = { ArrowLeft: cur - 1, ArrowRight: cur + 1, Home: 0, End: days.length - 1 }[e.key];
    if (next === undefined) return;
    e.preventDefault();
    show(next);
  });
  show(cur);
  fig.append(cap, svg, out);
  return fig;
}
