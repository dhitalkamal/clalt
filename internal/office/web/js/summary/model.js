// the summary reply as a view model: strings ready to show. the panel and the
// copy as text both read it, so they never disagree. no dom here.
import { fmtTokens, fmtCost, fmtMinutes } from './format.js';

const list = (v) => (Array.isArray(v) ? v : []);
const num = (v) => (Number.isFinite(v) ? v : 0);

// name x2, other x1. withOne false drops the x1 (files)
export function countList(items, withOne = true) {
  return list(items).map((c) => (c.count === 1 && !withOne ? c.name : c.name + ' x' + c.count)).join(', ');
}

export function plural(n, word) { return n + ' ' + word + (n === 1 ? '' : 's'); }

export function facts(s) {
  s = s || {};
  return [['Sessions', String(num(s.sessions))], ['Active', fmtMinutes(num(s.activeMinutes))], ['Tokens', fmtTokens(num(s.tokens))], ['Cost', fmtCost(num(s.cost))]];
}

// one line: 1 session, 5m active, 540 tokens, $0.00
export function factLine(s) {
  s = s || {};
  return [plural(num(s.sessions), 'session'), fmtMinutes(num(s.activeMinutes)) + ' active', fmtTokens(num(s.tokens)) + ' tokens', fmtCost(num(s.cost))].join(', ');
}

export function failureText(f) {
  return f.tool + ': ' + f.text + (f.count > 1 ? ' (x' + f.count + ')' : '');
}

// 5h limit: peak 70%, now 35%. empty when the day has no reading
export function limitLine(a) {
  a = a || {};
  const pct = (v) => Math.round(v) + '%';
  if (!Number.isFinite(a.peak5hPct)) return '';
  return '5h limit: peak ' + pct(a.peak5hPct) + (Number.isFinite(a.last5hPct) ? ', now ' + pct(a.last5hPct) : '');
}

function section(title, s) {
  return {
    title, limit: limitLine(s), peak5h: Number.isFinite(s.peak5hPct) ? s.peak5hPct : null,
    now5h: Number.isFinite(s.last5hPct) ? s.last5hPct : null, facts: facts(s), line: factLine(s),
    subagents: countList(s.subagents), tools: countList(s.tools), files: countList(s.files, false),
    failureCount: num(s.failureCount), failures: list(s.failures).map(failureText),
  };
}

export function summaryModel(sum) {
  const accounts = list(sum && sum.accounts).map((a) => ({
    id: a.id, ...section(a.label || a.id, a), projects: list(a.projects).map((p) => section(p.name, p)),
  }));
  return {
    day: sum && sum.day, truncated: !!(sum && sum.truncated), empty: accounts.length === 0,
    headline: facts(sum && sum.totals), line: factLine(sum && sum.totals), accounts,
  };
}
