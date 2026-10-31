// pure models of the PR reviews: the inbox, the findings of one review and the settings section. no dom.
import { age } from '../dom.js';

export const SEVERITIES = ['high', 'medium', 'low'];
const REPO = /^[A-Za-z0-9_.-]{1,100}\/[A-Za-z0-9_.-]{1,100}$/;

export const badgeText = (n) => (n > 99 ? '99+' : String(n));

// reviews the open tab shows, for the badge: merged and closed pull requests are history
export const openCount = (reply) => list(reply && reply.reviews).filter((r) => r.state !== 'merged' && r.state !== 'closed').length;

// the inbox button shows when reviews are on, or when old results are still there
export const showButton = (r) => !!r && (r.enabled === true || r.count > 0);

const list = (v) => (Array.isArray(v) ? v : []);
const sevOf = (s) => (SEVERITIES.includes(s) ? s : 'low');

function summaryOf(row) {
  if (row.error) return row.error;
  if (row.skipped) return row.skipped;
  const c = row.counts || {};
  const parts = SEVERITIES.filter((s) => c[s] > 0).map((s) => c[s] + ' ' + s);
  return parts.length ? parts.join(', ') : 'no findings';
}

function toneOf(row) {
  if (row.error) return 'error';
  if (row.skipped) return 'skipped';
  const c = row.counts || {};
  return SEVERITIES.find((s) => c[s] > 0) || 'clean';
}

const pad = (n) => String(n).padStart(2, '0');

// the attempt cap comes from the reply. anything but a positive integer means unknown, so no max is shown
const capOf = (reply) => (reply && Number.isInteger(reply.maxAttempts) && reply.maxAttempts > 0 ? reply.maxAttempts : 0);

// retry state of a failed review: only while the server still has a retry pending (nextRetryAt set)
function retryOf(row, max) {
  if (!row.error || !row.nextRetryAt) return '';
  const t = new Date(row.nextRetryAt);
  if (Number.isNaN(t.getTime())) return '';
  const next = (Number.isInteger(row.attempts) && row.attempts > 0 ? row.attempts : 1) + 1;
  return 'retrying at ' + pad(t.getHours()) + ':' + pad(t.getMinutes()) + ' (attempt ' + next + (max ? ' of ' + max : '') + ')';
}

// rows of the inbox, newest first. now is epoch ms
export function inboxRows(reply, now) {
  const max = capOf(reply);
  const rows = list(reply && reply.reviews).map((r) => ({
    id: r.id, title: r.title || '(no title)', ref: r.repo + '#' + r.number, author: r.author || '-', handle: r.handle || '',
    when: age(Math.max(0, now - Date.parse(r.endedAt))), at: Date.parse(r.endedAt) || 0,
    tone: toneOf(r), summary: summaryOf(r), retry: retryOf(r, max), account: r.account || '',
  })).map((r) => ({ ...r, tip: r.title + ' (' + r.ref + ')' }));
  return rows.sort((a, b) => b.at - a.at || (a.id < b.id ? -1 : 1));
}

// the two tabs. history is merged and closed rows with a badge and the utc day, anything else is open
export function tabRows(reply, now) {
  const state = new Map(list(reply && reply.reviews).map((r) => [r.id, r.state]));
  const out = { open: [], history: [] };
  for (const r of inboxRows(reply, now)) {
    const s = state.get(r.id);
    if (s === 'merged' || s === 'closed') out.history.push({ ...r, state: s, badge: s.toUpperCase(), date: new Date(r.at).toISOString().slice(0, 10) });
    else out.open.push({ ...r, state: s === 'open' ? s : 'unknown', badge: '' });
  }
  return out;
}

// the tab button labels with the size of each list, capped like the badge
export const tabLabels = (tabs) => ({
  open: 'Open (' + badgeText(list(tabs && tabs.open).length) + ')',
  history: 'History (' + badgeText(list(tabs && tabs.history).length) + ')',
});

export const whereText = (f) => (f.file ? (f.line > 0 ? f.file + ':' + f.line : f.file) : '');

// findings grouped by severity, high first, empty groups left out
export function groupFindings(findings) {
  const groups = SEVERITIES.map((severity) => ({ severity, items: [] }));
  for (const f of list(findings)) {
    groups[SEVERITIES.indexOf(sevOf(f.severity))].items.push({ where: whereText(f), title: f.title || '', detail: f.detail || '' });
  }
  return groups.filter((g) => g.items.length);
}

// the record the activity panel opens for the run log of a review, null when the run has no session
export function runLogRec(r) {
  if (!r || !r.handle) return null;
  return { role: 'review', handle: r.handle, title: 'Review ' + r.repo + '#' + r.number, account: r.account || '', project: r.repo, state: 'ended' };
}

export function lastCheckedText(iso, now) {
  const t = Date.parse(iso || '');
  if (!t) return 'not checked yet';
  const a = age(Math.max(0, now - t));
  return a === 'just now' ? 'checked just now' : 'checked ' + a + ' ago';
}

// the settings section: on/off and every repo that is opted in or suggested
export function settingsModel(view, now = Date.now()) {
  const on = new Set(list(view && view.repos));
  const names = [...new Set([...on, ...list(view && view.suggestions)])].sort();
  return { enabled: !!(view && view.enabled), repos: names.map((name) => ({ name, on: on.has(name) })), lastChecked: lastCheckedText(view && view.lastChecked, now) };
}

export const setEnabled = (m, enabled) => ({ ...m, enabled });
export const toggleRepo = (m, name, on) => ({ ...m, repos: m.repos.map((r) => (r.name === name ? { ...r, on } : r)) });

// adds a repo typed by hand, opted in. an existing name is just switched on
export function addRepo(m, text) {
  const name = String(text || '').trim();
  if (!REPO.test(name) || name.startsWith('.') || name.includes('..')) return { model: m, error: 'a repository is written owner/name' };
  if (m.repos.some((r) => r.name === name)) return { model: toggleRepo(m, name, true), error: '' };
  const repos = [...m.repos, { name, on: true }].sort((a, b) => (a.name < b.name ? -1 : 1));
  return { model: { ...m, repos }, error: '' };
}

export const saveBody = (m) => ({ enabled: m.enabled, repos: m.repos.filter((r) => r.on).map((r) => r.name) });
