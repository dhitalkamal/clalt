// the page's calls for PR reviews. same credential as the snapshot (the cookie); the save is a write, so it
// carries the edit header the server wants.
const TIMEOUT = 8000;

async function call(path, init) {
  const ctl = new AbortController();
  const timer = setTimeout(() => ctl.abort(), TIMEOUT);
  try {
    const res = await fetch(path, { cache: 'no-store', credentials: 'same-origin', signal: ctl.signal, ...init });
    if (!res.ok) {
      let text = '';
      try { text = (await res.text()).trim(); } catch (e) { /* the status says enough */ }
      throw new Error(text || 'http ' + res.status);
    }
    return await res.json();
  } finally {
    clearTimeout(timer);
  }
}

export const fetchReviews = () => call('/api/reviews');
export const fetchReview = (id) => call('/api/reviews/' + encodeURIComponent(id));
export const fetchSettings = () => call('/api/reviews/settings');
export const saveSettings = (body) => call('/api/reviews/settings', {
  method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Clalt-Edit': '1' }, body: JSON.stringify(body),
});
