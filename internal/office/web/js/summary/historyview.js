// the history tab body: per account a tokens chart and a busiest-5h chart.
import { el } from '../dom.js';
import { drawChart } from './chartsvg.js';

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
    sec.append(drawChart({ title: 'Tokens per day', who: a.label, days: a.days, key: 'tokens', cls: 'tokens', width }));
    sec.append(drawChart({ title: 'Busiest 5h window per day', who: a.label, days: a.days, key: 'peak5hTokens', cls: 'peak', width }));
    root.append(sec);
  }
  root.append(el('p', 'quiet', 'Tokens are input plus output. The 5h and 7d percent is only known for now: Claude Code reports no older readings, so past days show tokens.'));
  return root;
}
