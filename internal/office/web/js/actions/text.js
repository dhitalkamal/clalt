// confirm sentences and the requests of the session actions. no dom.
const JSON_HEADERS = { 'Content-Type': 'application/json', 'X-Clalt-Edit': '1' };
const LOST = 'this page lost its access, restart clalt office and open the new link';

export const confirmStop = (project, account) => 'Stop ' + project + ' on ' + account + '? It ends after the current step.';

export function confirmMoveAll(n, from, target) {
  const one = n === 1;
  return 'Move ' + n + (one ? ' session' : ' sessions') + ' from ' + from + ' to ' + target.label + ' (' + target.pct + '%)? ' + (one ? 'It moves' : 'Each moves') + ' at its next pause.';
}

export function moveRequest(verb, body) {
  return { url: '/api/session/' + verb, init: { method: 'POST', headers: JSON_HEADERS, credentials: 'same-origin', body: JSON.stringify(body) } };
}

// posts one action. resolves {ok: true} or {ok: false, reason}, never throws
export async function sendAction(verb, body, fetchImpl = fetch) {
  const r = moveRequest(verb, body);
  let res;
  try { res = await fetchImpl(r.url, r.init); } catch { return { ok: false, reason: 'clalt office is not reachable' }; }
  if (res.ok) return { ok: true };
  if (res.status === 401 || res.status === 403) return { ok: false, reason: LOST };
  let reason = '';
  try { const j = await res.json(); reason = j && typeof j.reason === 'string' ? j.reason : ''; } catch { /* no body */ }
  return { ok: false, reason: reason || 'http ' + res.status };
}
