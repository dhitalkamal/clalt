// the "full in ~25m" line of an account card, from the snapshot's forecast. pure.
const HOUR = 60;
const RED_UNDER = 20;

// null when there is nothing to warn about yet (no forecast, or an hour or more left)
export function forecastView(fc, now) {
  const at = fc && fc.fiveHourAt ? Date.parse(fc.fiveHourAt) : NaN;
  if (Number.isNaN(at)) return null;
  const left = (at - now) / 60000;
  if (left >= HOUR) return null;
  if (left <= 0) return { text: 'full now', level: 'red', minutes: 0 };
  const minutes = Math.ceil(left);
  return { text: 'full in ~' + minutes + 'm', level: minutes < RED_UNDER ? 'red' : 'amber', minutes };
}
