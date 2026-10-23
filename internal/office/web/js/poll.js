// fetches the snapshot about once a second. the cookie from the first page load is the credential.
const EVERY = 1000;
const HIDDEN_EVERY = 5000;
const TIMEOUT = 4000;
const MAX_BACKOFF = 30000;

export async function fetchSnapshot() {
  const ctl = new AbortController();
  const timer = setTimeout(() => ctl.abort(), TIMEOUT);
  try {
    const res = await fetch('/api/snapshot', { cache: 'no-store', credentials: 'same-origin', signal: ctl.signal });
    if (res.status === 401 || res.status === 403) throw new Error('auth');
    if (!res.ok) throw new Error('http ' + res.status);
    return await res.json();
  } finally {
    clearTimeout(timer);
  }
}

// calls onSnapshot for each good answer and onError for each failure. a failure waits 1 s, then 2 s and so on
// up to 30 s before the next try, and a good answer starts over. a refused token stops the polling for good.
export function startPoll(onSnapshot, onError) {
  let fails = 0;
  const tick = async () => {
    try {
      onSnapshot(await fetchSnapshot());
      fails = 0;
    } catch (e) {
      onError(e);
      if (e.message === 'auth') return;
      fails++;
    }
    const wait = fails ? Math.min(MAX_BACKOFF, EVERY * 2 ** (fails - 1)) : document.hidden ? HIDDEN_EVERY : EVERY;
    setTimeout(tick, wait);
  };
  tick();
}
