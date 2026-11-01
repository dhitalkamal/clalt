// the one fetch the panel makes: GET /api/activity. same credential as the snapshot (the cookie).
import { LOST_ACCESS } from '../lostaccess.js';
const TIMEOUT = 8000;

export async function fetchActivity(path) {
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

// one line for the error strip
export function errorText(e) {
  const m = e && e.message ? e.message : String(e);
  if (m === 'auth') return LOST_ACCESS;
  if (e && e.status === 404) return 'no transcript for this session yet';
  if (e && e.status === 400) return 'the activity request was refused (bad parameters)';
  if (e && e.name === 'AbortError') return 'clalt did not answer in time';
  return 'could not load the activity: ' + m;
}
