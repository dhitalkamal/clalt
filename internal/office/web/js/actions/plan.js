import { isCopilot } from '../tool.js';

// picks accounts for a move. pure: accounts is the page's Map of id -> {label, limits, forecast}.
const used = (a) => {
  const w = a.limits && a.limits.fiveHour;
  if (w) return Math.round(w.usedPct);
  // a reading with no 5h window means the window has reset: all of it is free. no reading at all is unknown
  return a.limits && a.limits.readAt ? 0 : null;
};
const soon = (a) => !!(a.forecast && a.forecast.fiveHourAt);

// the other accounts, best first: known room before unknown, accounts that run out themselves last
export function rankAccounts(accounts, excludeId) {
  const list = [];
  for (const [id, a] of accounts) if (id !== excludeId && !isCopilot({ id, tool: a.tool })) list.push({ id, label: a.label || id, pct: used(a), soon: soon(a) });
  const key = (x) => (x.soon ? 2 : x.pct === null ? 1 : 0);
  return list.map((x, i) => [x, i]).sort(([a, i], [b, j]) => key(a) - key(b) || (a.pct ?? 0) - (b.pct ?? 0) || i - j).map(([x]) => x);
}

// every movable session of the account and where they go. target is null when there is nowhere good to go
export function planMoveAll(sessions, fromId, accounts) {
  const mine = sessions.filter((s) => s.account === fromId && !s.leaving);
  const handles = mine.filter((s) => s.actions && s.actions.move).map((s) => s.handle);
  const best = rankAccounts(accounts, fromId)[0];
  const target = best && !best.soon && best.pct !== null ? best : null;
  return { target, handles, skipped: mine.length - handles.length };
}
