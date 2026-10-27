// the maths of the percent history charts: a fixed 0 to 100 scale with a guide
// at 90 percent. days with no reading are gaps, never zero. no dom, so node tests it.
import { frame } from './chart.js';
import { shortDay } from './format.js';

export const PCT_GUIDE = 90;
const TICKS = [0, 25, 50, 75, 100];
const clamp = (v) => Math.max(0, Math.min(100, v));
const has = (d, key) => Number.isFinite(d[key]);

// bars for the day field key, in the same frame as the token charts
export function pctModel({ days, key, width, height }) {
  const { plot, slot, bw, xTicks } = frame(days, width, height);
  const at = (v) => plot.y + plot.h - (v / 100) * plot.h;
  const bars = days.map((d, i) => {
    const missing = !has(d, key);
    const value = missing ? 0 : clamp(d[key]);
    const h = missing ? 0 : Math.max(value > 0 ? 1 : 0, (value / 100) * plot.h);
    return { i, day: d.day, value, missing, hot: !missing && d[key] > PCT_GUIDE, x: plot.x + i * slot + (slot - bw) / 2, w: bw, h, y: plot.y + plot.h - h };
  });
  const yTicks = TICKS.map((value) => ({ value, label: value + '%', y: at(value) }));
  return { plot, top: 100, bars, yTicks, xTicks, guide: { value: PCT_GUIDE, y: at(PCT_GUIDE) } };
}

// one day read aloud: name is 5h or 7d
export function pctReadout(d, key, name) {
  const label = shortDay(d.day) + ': ';
  if (!has(d, key)) return label + 'no ' + name + ' reading';
  const over = d[key] > PCT_GUIDE ? ', over the ' + PCT_GUIDE + '% line' : '';
  return label + name + ' peak ' + Math.round(d[key]) + '%' + over;
}

// whether any day has a reading, the first that does, and whether days come before it
export function pctSeries(days, key) {
  const i = days.findIndex((d) => has(d, key));
  return { any: i >= 0, firstDay: i >= 0 ? days[i].day : null, gaps: days.length > 0 && i !== 0 };
}

// the note under the percent charts: only when some days have no reading before the first one
export function pctNote(days, keys) {
  const s = keys.map((k) => pctSeries(days, k));
  if (!days.length || !s.some((x) => x.gaps)) return '';
  const firsts = s.filter((x) => x.any).map((x) => x.firstDay).sort();
  if (!firsts.length) return 'No percent readings yet. clalt logs them while claude runs.';
  return 'Percent readings began on ' + shortDay(firsts[0]) + '. Earlier days show tokens only.';
}
