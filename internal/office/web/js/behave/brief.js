// who briefs a new subagent. pure, no dom, so node tests can drive it.

// live subagent ids per session that is not leaving
export function subIdsOf(sessions) {
  const out = new Map();
  for (const s of sessions) if (!s.leaving) out.set(s.id, new Set(s.subs.filter((x) => !x.leaving).map((x) => x.id)));
  return out;
}

// the briefs to start: each subagent that is new to a session the page already knew.
// the first snapshot and a session that is itself new start none
export function newBriefs(prev, cur, first) {
  const out = [];
  if (first) return out;
  for (const [main, ids] of cur) {
    const was = prev.get(main);
    if (!was) continue;
    for (const sub of ids) if (!was.has(sub)) out.push({ main, sub });
  }
  return out;
}

// a main agent may get up for a brief only when it sits calmly at its desk
export function briefReady(a, calm) {
  if (calm || a.mode !== 'seated' || !a.briefQueue || !a.briefQueue.length) return false;
  return a.s.state !== 'needs-you' && a.s.state !== 'ended' && !a.s.leaving;
}
