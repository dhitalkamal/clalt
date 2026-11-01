// the two reads the summary panel makes. same credential as the snapshot (the cookie).
import { validDay } from './format.js';
import { LOST_ACCESS } from '../lostaccess.js';

const TIMEOUT = 20000;

async function getJson(path) {
  const ctl = new AbortController();
  const timer = setTimeout(() => ctl.abort(), TIMEOUT);
  try {
    const res = await fetch(path, { cache: 'no-store', credentials: 'same-origin', signal: ctl.signal });
    if (res.status === 401 || res.status === 403) throw new Error('auth');
    if (!res.ok) throw Object.assign(new Error('http ' + res.status), { status: res.status });
    return await res.json();
  } finally {
    clearTimeout(timer);
  }
}

export function fetchSummary(day) {
  if (!validDay(day)) return Promise.reject(new Error('bad day'));
  return getJson('/api/summary?day=' + day);
}

export function fetchHistory(days) {
  return getJson('/api/history?days=' + Math.max(1, Math.min(90, Math.floor(days) || 30)));
}

// one line for the error strip
export function errorText(e) {
  const m = e && e.message ? e.message : String(e);
  if (m === 'auth') return LOST_ACCESS;
  if (e && e.status === 404) return 'this clalt has no summary to give';
  if (e && e.name === 'AbortError') return 'clalt did not answer in time, reading the transcripts takes a while on big histories';
  return 'could not load the summary: ' + m;
}
