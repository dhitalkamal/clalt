// one short line for an account card: how many sessions work, wait for you, idle or are on break. pure.
// sessions: [{state, away}], away is true while an idle session is out of its cabin (cafe, play area)
export function accountLine(sessions) {
  const n = { need: 0, work: 0, idle: 0, brk: 0 };
  for (const s of sessions) {
    if (s.state === 'needs-you') n.need++;
    else if (s.state === 'busy') n.work++;
    else if (s.away) n.brk++;
    else n.idle++;
  }
  const parts = [[n.need, 'needs you'], [n.work, 'working'], [n.idle, 'idle'], [n.brk, 'on break']];
  return parts.filter(([c]) => c > 0).map(([c, t]) => c + ' ' + t).join(', ');
}
