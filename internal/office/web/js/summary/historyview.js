// the history tab body: per account the 5h and 7d peak percent charts, then the
// tokens chart and the busiest-5h chart.
import { el } from '../dom.js';
import { drawChart } from './chartsvg.js';
import { pctModel, pctReadout, pctSeries, pctNote } from './pctchart.js';

const HEIGHT = 170;
const PCT = [['peak5hPct', '5h', '5h peak percent per day', 'pct5'], ['peak7dPct', '7d', '7d peak percent per day', 'pct7']];

const hasUsage = (a) => a.days.some((d) => d.tokens > 0);

// hist is the /api/history reply, width the chart width in pixels
export function renderHistory(hist, width) {
  const root = el('div', 'histview');
  const accounts = (hist && Array.isArray(hist.accounts) ? hist.accounts : []).filter((a) => Array.isArray(a.days));
  if (!accounts.length) {
    root.append(el('p', 'quiet', 'No accounts to show.'));
    return root;
  }
  if (hist.truncated) root.append(el('p', 'warn', 'Some transcripts were not read (size limit), so the oldest days may be low.'));
  if (!accounts.some(hasUsage)) root.append(el('p', 'quiet', 'No usage in the last ' + hist.days + ' days.'));
  for (const a of accounts) {
    const sec = el('section', 'acct', el('h3', '', a.label || a.id));
    for (const [key, name, title, cls] of PCT) {
      if (!pctSeries(a.days, key).any) continue;
      const model = pctModel({ days: a.days, key, width, height: HEIGHT });
      sec.append(drawChart({ title, who: a.label, days: a.days, key, cls: 'pct ' + cls, width, model, read: (d) => pctReadout(d, key, name) }));
    }
    const note = pctNote(a.days, PCT.map((p) => p[0]));
    if (note) sec.append(el('p', 'quiet', note));
    sec.append(drawChart({ title: 'Tokens per day', who: a.label, days: a.days, key: 'tokens', cls: 'tokens', width }));
    sec.append(drawChart({ title: 'Busiest 5h window per day', who: a.label, days: a.days, key: 'peak5hTokens', cls: 'peak', width }));
    root.append(sec);
  }
  root.append(el('p', 'quiet', 'Percent is the highest reading of the day from the limits clalt logged, with a line at 90%. Tokens are input plus output.'));
  return root;
}
