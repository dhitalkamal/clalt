// the +N badge over a team desk: how many extra interns of that member have no stool. counts live here, the
// label layer (labels.js) draws them as css pills. pure state, no three.js.
const MAX = 99;
let counts = new Map();
let notify = null;

export function onBadgeChange(fn) { notify = fn; }

export function resetBadges() { counts = new Map(); synced = new Set(); }

export const badgeCount = (id) => counts.get(id) || 0;

// n extra interns over a desk, 0 hides the badge. fractions and negatives clamp, the pill stops at 99
export function setDeskBadge(memberId, n) {
  const v = Math.max(0, Math.min(MAX, Math.floor(Number(n) || 0)));
  if (v === badgeCount(memberId)) return;
  if (v) counts.set(memberId, v); else counts.delete(memberId);
  if (notify) notify();
}

// app.badges: Map teamId -> hidden intern count. sets every listed member, clears the ones it set before
let synced = new Set();
export function syncBadges(map) {
  const now = new Set();
  if (map) for (const [id, n] of map) { setDeskBadge(id, n); if (badgeCount(id)) now.add(id); }
  for (const id of synced) if (!now.has(id)) setDeskBadge(id, 0);
  synced = now;
}

// label items for the members that have a count and a desk seat
export function badgeItems(seats) {
  const out = [];
  for (const s of seats) {
    if (s.kind !== 'desk' || s.member === undefined) continue;
    const n = badgeCount(s.member);
    if (!n) continue;
    out.push({ id: 'badge-' + s.member, cls: 'badge', key: '+' + n, x: s.x, y: 1.05, z: s.z, render: (d) => { d.textContent = '+' + n; } });
  }
  return out;
}
