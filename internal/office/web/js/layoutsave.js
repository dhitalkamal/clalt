// saves the layout from the editor with the version it was loaded at (If-Match), so a save
// never overwrites what another tab saved meanwhile. a 409 becomes an error with conflict and theirs set.
const JSON_HEADERS = { 'Content-Type': 'application/json', 'X-Clalt-Edit': '1' };

async function oneLine(res) {
  let text = '';
  try { text = (await res.text()).trim().split('\n')[0]; } catch (e) { /* the status says enough */ }
  return text || 'http ' + res.status;
}

export function createLayoutSaver(fetchFn) {
  const call = (url, init) => (fetchFn || globalThis.fetch)(url, { credentials: 'same-origin', ...init });
  let version = null;

  const tagOf = (res) => (res.headers && res.headers.get && res.headers.get('ETag')) || null;
  const adopt = (res) => { version = tagOf(res) || version; };

  async function refresh() {
    const res = await call('/api/layout', { cache: 'no-store' });
    if (!res.ok) throw new Error(await oneLine(res));
    adopt(res);
    return res.json();
  }

  async function save(layout) {
    const headers = version ? { ...JSON_HEADERS, 'If-Match': version } : JSON_HEADERS;
    const res = await call('/api/layout', { method: 'PUT', headers, body: JSON.stringify(layout) });
    if (res.ok) { adopt(res); return; }
    if (res.status === 409) {
      const text = (await res.text()).trim();
      let body = null;
      try { body = JSON.parse(text); } catch (e) { /* a plain text refusal */ }
      if (body && body.layout) {
        version = tagOf(res) || (body.version ? '"' + body.version + '"' : version);
        throw Object.assign(new Error('This layout changed in another tab.'), { conflict: true, theirs: body.layout });
      }
      throw new Error(text.split('\n')[0] || 'http 409');
    }
    throw new Error(await oneLine(res));
  }

  return { version: () => version, refresh, save, adopt };
}
