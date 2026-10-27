// settings model: what the dialog lists, what it sends and what it says when that fails. no dom.
const JSON_HEADERS = { 'Content-Type': 'application/json', 'X-Clalt-Edit': '1' };
export const SAVE_LABEL = 'Set as default';
const PREFIX = 'could not save the default office: ';

// the reply of GET /api/layouts: templates first as the server orders them, then saved layouts.
// a name is listed once, the current one is checked.
export function layoutOptions(reply) {
  let active = reply && typeof reply.active === 'string' ? reply.active : '';
  if (active === 'default') active = 'hall';
  const options = [];
  const seen = new Set();
  const add = (id, title, group) => {
    if (typeof id !== 'string' || !id || seen.has(id)) return;
    seen.add(id);
    options.push({ id, title: title || id, group, checked: id === active, isDefault: id === active });
  };
  const list = (v) => (Array.isArray(v) ? v : []);
  for (const t of list(reply && reply.templates)) if (t) add(t.id, t.title, 'template');
  // builtin files are the template sources (default is the hall), listing them would show those twice
  for (const l of list(reply && reply.layouts)) if (l && !l.builtin) add(l.name, l.title, 'saved');
  return { active, options };
}

export const confirmText = (title) => title + ' is now your default. It opens every time you run clalt office.';

// nothing to set when no option is picked or the pick already is the default
export const canSetDefault = (picked, current) => !!picked && picked !== current;

export function saveRequest(id) {
  return {
    url: '/api/layout/use',
    init: { method: 'POST', headers: JSON_HEADERS, credentials: 'same-origin', body: JSON.stringify({ name: id }) },
  };
}

// one line for the error row. text is the server's one line body, status the http code
export function errorText({ status, text, network } = {}) {
  if (network) return PREFIX + 'clalt office is not reachable';
  return PREFIX + ((text || '').trim() || 'http ' + status);
}

// resolves to the reply body, throws an Error whose message is ready to show
export async function saveDefault(id, fetchFn = fetch) {
  const { url, init } = saveRequest(id);
  let res;
  try { res = await fetchFn(url, init); } catch (e) { throw new Error(errorText({ network: true })); }
  if (!res.ok) {
    let text = '';
    try { text = await res.text(); } catch (e) { /* no body, the status says enough */ }
    throw new Error(errorText({ status: res.status, text }));
  }
  return res.json();
}
