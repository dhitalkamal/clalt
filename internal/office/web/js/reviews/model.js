// pure models of the PR reviews: the inbox, the findings of one review and the settings section. no dom.
import { age } from '../dom.js';

export const SEVERITIES = ['high', 'medium', 'low'];
const REPO = /^[A-Za-z0-9_.-]{1,100}\/[A-Za-z0-9_.-]{1,100}$/;

export const badgeText = (n) => (n > 99 ? '99+' : String(n));

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

// rows of the inbox, newest first. now is epoch ms
export function inboxRows(reply, now) {
  const rows = list(reply && reply.reviews).map((r) => ({
    id: r.id, title: r.title || '(no title)', ref: r.repo + '#' + r.number, author: r.author || '-', handle: r.handle || '',
    when: age(Math.max(0, now - Date.parse(r.endedAt))), at: Date.parse(r.endedAt) || 0,
    tone: toneOf(r), summary: summaryOf(r), account: r.account || '',
  }));
  return rows.sort((a, b) => b.at - a.at || (a.id < b.id ? -1 : 1));
}

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
