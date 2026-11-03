// one short line for an account card: how many sessions work, wait for you, idle or are on break or resting. pure.
// sessions: [{state, away, rest}], away is true while an idle session is out of its cabin, rest when it went to
// a room chosen in the idle settings (others are on a cafe or play area break)
export function accountLine(sessions) {
  const n = { need: 0, work: 0, idle: 0, brk: 0, rest: 0 };
  for (const s of sessions) {
    if (s.state === 'needs-you') n.need++;
    else if (s.state === 'busy') n.work++;
    else if (s.away && s.rest) n.rest++;
    else if (s.away) n.brk++;
    else n.idle++;
  }
  const parts = [[n.need, 'needs you'], [n.work, 'working'], [n.idle, 'idle'], [n.rest, 'resting'], [n.brk, 'on break']];
  return parts.filter(([c]) => c > 0).map(([c, t]) => c + ' ' + t).join(', ');
}
