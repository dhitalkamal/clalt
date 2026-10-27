// the maths of the history charts: ticks, bars, labels. no dom, so node tests it.
import { fmtTokens, fmtCost, shortDay } from './format.js';

// the plot area's margins inside the svg: room for y labels on the left, x labels below
const M = { left: 46, right: 8, top: 8, bottom: 22 };
const LABEL_W = 40; // pixels one x label needs

// about four steps of 1, 2 or 5 times a power of ten, topping at or over max
export function niceTicks(max) {
  const v = Number.isFinite(max) && max > 0 ? max : 0;
  if (v === 0) return { top: 10, ticks: [0, 5, 10] };
  const raw = v / 4;
  const k = Math.pow(10, Math.floor(Math.log10(raw)));
  const m = raw / k;
  const step = Math.max(1, (m < 1.5 ? 1 : m < 3 ? 2 : m < 7 ? 5 : 10) * k);
  const n = Math.ceil(v / step);
  const ticks = [];
  for (let i = 0; i <= n; i++) ticks.push(i * step);
  return { top: n * step, ticks };
}

// which days get an x label: every step-th, counted back from the last so it is always there
export function dayTicks(days, maxLabels) {
  const n = days.length;
  if (!n) return [];
  const step = Math.ceil(n / Math.max(1, maxLabels));
  const out = [];
  for (let i = n - 1; i >= 0; i -= step) out.push({ i, text: shortDay(days[i]) });
  return out.reverse();
}

// bars, ticks and the plot box for width x height pixels. key is the day field to draw
export function chartModel({ days, key, width, height }) {
  const plot = { x: M.left, y: M.top, w: Math.max(10, width - M.left - M.right), h: Math.max(10, height - M.top - M.bottom) };
  const vals = days.map((d) => (Number.isFinite(d[key]) && d[key] > 0 ? d[key] : 0));
  const { top, ticks } = niceTicks(Math.max(0, ...vals));
  const n = days.length;
  const slot = n ? plot.w / n : plot.w;
  const bw = Math.max(1, slot * 0.7);
  const bars = vals.map((value, i) => {
    let h = (value / top) * plot.h;
    if (value > 0 && h < 1) h = 1;
    return { i, day: days[i].day, value, x: plot.x + i * slot + (slot - bw) / 2, w: bw, h, y: plot.y + plot.h - h };
  });
  const yTicks = ticks.map((value) => ({ value, label: fmtTokens(value), y: plot.y + plot.h - (value / top) * plot.h }));
  const xTicks = dayTicks(days.map((d) => d.day), Math.floor(plot.w / LABEL_W)).map((t) => ({ ...t, x: plot.x + t.i * slot + slot / 2 }));
  return { plot, top, bars, yTicks, xTicks };
}

// one day read aloud, for the line under a chart and for screen readers
export function readout(d) {
  const label = shortDay(d.day) + ': ';
  if (!(d.tokens > 0) && !(d.cost > 0)) return label + 'no usage';
  const parts = [fmtTokens(d.tokens) + ' tokens', fmtCost(d.cost), 'busiest 5h ' + fmtTokens(d.peak5hTokens), 'busiest 7d ' + fmtTokens(d.peak7dTokens)];
  const now = [];
  if (Number.isFinite(d.peak5hPct)) now.push('5h ' + Math.round(d.peak5hPct) + '%');
  if (Number.isFinite(d.peak7dPct)) now.push('7d ' + Math.round(d.peak7dPct) + '%');
  if (now.length) parts.push('now ' + now.join(', '));
  return label + parts.join(', ');
}
