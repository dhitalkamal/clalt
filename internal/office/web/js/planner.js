// decides who takes a break and where. runs every step, only reads state and timers. the places come from the
// break spots of the layout (L.groups): counters and tables in the cafe, ping-pong tables, arcades, bean bags, coolers.
const SOLO = ['cooler', 'arcade', 'read', 'carry'];

const names = (L, group) => (L.groups[group] || []).map((s) => s.name);
const pairs = (L, group) => {
  const seen = new Set();
  const out = [];
  for (const s of L.groups[group] || []) {
    if (!s.partner || seen.has(s.name)) continue;
    seen.add(s.name); seen.add(s.partner);
    out.push([s.name, s.partner]);
  }
  return out;
};

const order = (spot, act, dur) => ({ spot, act, dur });

// two agents chat over a coffee at one table, or play ping-pong
const steps = {
  chat: (L, free) => {
    const table = pairs(L, 'table').find(([a, b]) => free(a) && free(b));
    const counters = names(L, 'counter').filter(free);
    if (!table || counters.length < 2) return null;
    return [
      { steps: [order(counters[0], 'order', 3.4), order(table[0], 'sip', 20)], carry: false },
      { steps: [order(counters[1], 'order', 3.4), order(table[1], 'sip', 20)], carry: false },
    ];
  },
  pong: (L, free) => {
    const p = pairs(L, 'pong').find(([a, b]) => free(a) && free(b));
    return p ? [{ steps: [order(p[0], 'pong', 22)], carry: false }, { steps: [order(p[1], 'pong', 22)], carry: false }] : null;
  },
};

const first = (L, free, group) => names(L, group).find(free) || null;
const solo = {
  cooler: (L, free) => { const n = first(L, free, 'cooler'); return n ? { steps: [order(n, 'cooler', 7)], carry: false } : null; },
  arcade: (L, free) => { const n = first(L, free, 'arcade'); return n ? { steps: [order(n, 'arcade', 12)], carry: false } : null; },
  read: (L, free) => { const n = first(L, free, 'bag'); return n ? { steps: [order(n, 'read', 14)], carry: false } : null; },
  carry: (L, free) => { const n = first(L, free, 'counter'); return n ? { steps: [order(n, 'order', 3.4)], carry: true } : null; },
};

export function planBreaks(app, force = false) {
  const L = app.layout;
  const agents = [...app.agents.values()];
  const reserved = new Set();
  for (const a of agents) for (const s of (a.plan?.steps || a.planOffer?.steps || [])) reserved.add(s.spot);
  const free = (n) => !reserved.has(n);
  const take = (plan) => { for (const s of plan.steps) reserved.add(s.spot); };
  const ready = agents.filter((a) => a.s.state === 'idle' && !a.s.leaving && !a.plan && !a.planOffer && a.slot.kind !== 'stand'
    && (force ? a.mode !== 'away' : a.mode === 'seated' && app.time >= a.nextBreakAt));
  const offered = [];
  while (ready.length >= 2) {
    const a = ready.shift();
    const b = ready.shift();
    const kind = (app.pairCycle++ % 2) ? 'pong' : 'chat';
    const plans = steps[kind](L, free) || steps[kind === 'chat' ? 'pong' : 'chat'](L, free);
    if (!plans) { ready.unshift(b); break; }
    take(plans[0]); take(plans[1]);
    a.planOffer = plans[0]; b.planOffer = plans[1];
    offered.push(a, b);
  }
  for (const a of ready) {
    for (let i = 0; i < SOLO.length; i++) {
      const plan = solo[SOLO[((a.s.seed + (a.breakCount || 0)) + i) % SOLO.length]](L, free);
      if (plan) { take(plan); a.planOffer = plan; offered.push(a); break; }
    }
  }
  return offered;
}
