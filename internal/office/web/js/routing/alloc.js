// desk allocation shared by the hall (v1) and garden (v2) planners.
// agents arrive sorted by priority: working agents first, then idle ones. pool(a, got) lists the seats an
// agent may use in preference order; carryOk(a, s, isFree) can veto keeping an old desk; idle(a) says whether it is resting; evictable(a) whether a resting
// agent may be bumped from its desk. returns {got: Map(agentKey -> seat), displaced: [agentKey]}.
export function allocDesks({ agents, prev, seats, idOf, pool, idle, evictable, carryOk = () => true }) {
  const byId = new Map(seats.map((s) => [idOf(s), s]));
  const holder = new Map();
  const got = new Map();
  const displaced = [];
  const give = (a, s) => { holder.set(idOf(s), a); got.set(a.key, s); };
  const isFree = (s) => !holder.has(idOf(s));

  // seated agents keep their desk while it is still allowed, whatever their state
  for (const a of agents) {
    const old = prev?.get(a.key);
    const s = old?.kind === 'seat' ? byId.get(old.seat) : null;
    if (s && isFree(s) && pool(a, got).includes(s) && carryOk(a, s, isFree)) give(a, s);
  }

  // working agents without a desk: a free one, else bump a resting agent (never a working one)
  for (const a of agents) {
    if (idle(a) || got.has(a.key)) continue;
    const list = pool(a, got);
    let s = list.find(isFree);
    if (!s) {
      s = list.find((x) => { const h = holder.get(idOf(x)); return h && idle(h) && evictable(h); });
      if (s) { const h = holder.get(idOf(s)); got.delete(h.key); displaced.push(h.key); }
    }
    if (s) give(a, s);
  }

  // resting agents take whatever desk is still free
  for (const a of agents) {
    if (!idle(a) || got.has(a.key)) continue;
    const s = pool(a, got).find(isFree);
    if (s) give(a, s);
  }
  return { got, displaced };
}
