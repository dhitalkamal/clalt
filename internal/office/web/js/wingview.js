// what an account card shows. the Copilot team has no limits, so no bars and no percent.
// worst is the busiest window in percent or null, pct says whether the compact card shows a percent at all.
import { isCopilot } from './tool.js';

export function wingView(label, limits, id) {
  if (isCopilot({ id })) return { label, bars: false, worst: null, pct: false };
  const used = [limits && limits.fiveHour, limits && limits.sevenDay].filter(Boolean).map((w) => Math.round(w.usedPct));
  return { label, bars: true, worst: used.length ? Math.max(...used) : null, pct: true };
}
