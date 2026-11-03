// what a seated agent acts out for its current work, from the snapshot's doing field.
// pure: no dom, no clock of its own. the caller passes the time in milliseconds.

// how long a facepalm or a cheer lasts before the plain busy pose comes back
export const REACT_MS = 3000;

const SHORT = new Set(['pass', 'fail']);

// the act of one doing, or null when the tool has none. prevAct is what the agent
// was acting out before: a command that ends ok only cheers after a wait
function baseAct(doing, prevAct) {
  switch (doing && doing.kind) {
    case 'read': return 'read';
    case 'edit': return 'type';
    case 'web': return 'web';
    case 'run':
      if (doing.state === 'running') return 'wait';
      if (doing.state === 'fail') return 'fail';
      return doing.state === 'ok' && prevAct === 'wait' ? 'pass' : null;
    default: return null;
  }
}

const keyOf = (d) => [d.kind, d.tool, d.state, d.at].join('|');

// the next tracking record for an agent. prev is the one it returned last poll
// (or undefined), doing is the new field (or null). a doing seen before keeps
// its start time, so a short act is not restarted by every poll.
export function track(prev, doing, now) {
  if (!doing) return { key: '', act: null, since: now };
  const key = keyOf(doing);
  if (prev && prev.key === key) return prev;
  return { key, act: baseAct(doing, prev && prev.act), since: now };
}

// the act to show at time now: short acts run out
export function currentAct(tr, now) {
  if (!tr || !tr.act) return null;
  if (SHORT.has(tr.act) && now - tr.since > REACT_MS) return null;
  return tr.act;
}
