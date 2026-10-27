// one refresh tick: everything newer than the newest entry the panel has. the route answers an
// after request with the new entries newest first; when more than a page arrived it gives a next
// cursor, followed with before=<next> and the same after until it is empty.
// fetchPage(params) -> reply. returns {entries, newest, handle, agent, complete}. newest is what the
// first reply names, the next tick's after. complete is false when maxPages ran out first.
export async function fetchNewer(fetchPage, target, after, { limit = 100, maxPages = 10 } = {}) {
  const base = { handle: target.handle, agent: target.agent, after, limit };
  const first = await fetchPage(base);
  const entries = [...(first.entries || [])];
  let next = first.next || '';
  let pages = 1;
  while (next && pages < maxPages) {
    const r = await fetchPage({ ...base, before: next });
    entries.push(...(r.entries || []));
    next = r.next || '';
    pages++;
  }
  return { entries, newest: first.newest || after || '', handle: first.handle, agent: first.agent || '', complete: !next };
}
