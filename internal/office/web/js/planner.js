// decides who takes a break and where. runs every step, only reads state and timers.
const SOLO = ['cooler', 'arcade', 'read', 'carry'];

const steps = {
  chat: (a, b, free) => {
    const tables = free('tableW') && free('tableE') ? ['tableW', 'tableE'] : free('tableN') && free('tableS') ? ['tableN', 'tableS'] : null;
    if (!tables || !free('counterA') || !free('counterB')) return null;
    return [
      { steps: [{ spot: 'counterA', act: 'order', dur: 3.4 }, { spot: tables[0], act: 'sip', dur: 20 }], carry: false },
      { steps: [{ spot: 'counterB', act: 'order', dur: 3.4 }, { spot: tables[1], act: 'sip', dur: 20 }], carry: false },
    ];
  },
  pong: (a, b, free) => (free('pongA') && free('pongB') ? [
    { steps: [{ spot: 'pongA', act: 'pong', dur: 22 }], carry: false },
    { steps: [{ spot: 'pongB', act: 'pong', dur: 22 }], carry: false },
  ] : null),
};

const solo = {
  cooler: (free) => (free('cooler') && free('window') ? { steps: [{ spot: 'cooler', act: 'cooler', dur: 7 }, { spot: 'window', act: 'window', dur: 8 }], carry: false } : null),
  arcade: (free) => { const n = free('arcade1') ? 'arcade1' : free('arcade2') ? 'arcade2' : null; return n ? { steps: [{ spot: n, act: 'arcade', dur: 12 }], carry: false } : null; },
  read: (free) => { const n = free('bagA') ? 'bagA' : free('bagB') ? 'bagB' : null; return n ? { steps: [{ spot: n, act: 'read', dur: 14 }], carry: false } : null; },
  carry: (free) => { const n = free('counterA') ? 'counterA' : free('counterB') ? 'counterB' : null; return n ? { steps: [{ spot: n, act: 'order', dur: 3.4 }], carry: true } : null; },
};

export function planBreaks(app, force = false) {
  const agents = [...app.agents.values()];
  const reserved = new Set();
  for (const a of agents) for (const s of (a.plan?.steps || a.planOffer?.steps || [])) reserved.add(s.spot);
  const free = (n) => !reserved.has(n);
  const take = (plan) => { for (const s of plan.steps) reserved.add(s.spot); };
  const ready = agents.filter((a) => a.s.state === 'idle' && !a.s.leaving && !a.plan && !a.planOffer
    && (force ? a.mode !== 'away' : a.mode === 'seated' && app.time >= a.nextBreakAt));
  const offered = [];
  while (ready.length >= 2) {
    const a = ready.shift();
    const b = ready.shift();
    const kind = (app.pairCycle++ % 2) ? 'pong' : 'chat';
    const plans = steps[kind](a, b, free) || steps[kind === 'chat' ? 'pong' : 'chat'](a, b, free);
    if (!plans) { ready.unshift(b); break; }
    take(plans[0]); take(plans[1]);
    a.planOffer = plans[0]; b.planOffer = plans[1];
    offered.push(a, b);
  }
  for (const a of ready) {
    for (let i = 0; i < SOLO.length; i++) {
      const plan = solo[SOLO[((a.s.seed + (a.breakCount || 0)) + i) % SOLO.length]](free);
      if (plan) { take(plan); a.planOffer = plan; offered.push(a); break; }
    }
  }
  return offered;
}
