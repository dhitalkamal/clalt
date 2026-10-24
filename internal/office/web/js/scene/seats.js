// who sits where: asks the routing module for a plan, then turns it into the slots the agents walk to
import { planSeats } from '../routing.js';

const rosterOf = (sessions) => sessions.flatMap((s) => [
  { key: s.id, kind: 'main', account: s.account, type: 'main', state: s.state },
  ...s.subs.map((x) => ({ key: x.id, kind: 'sub', account: s.account, type: x.type, state: 'busy' })),
]);

// returns the slots by agent key and the raw plan (kept for the next call so seats stay put)
export function assignSeats(L, sessions, catalog, prev) {
  const plan = planSeats(L.layout, rosterOf(sessions), catalog, prev);
  const slots = new Map();
  for (const [key, e] of plan) {
    const item = e.kind === 'seat' ? L.itemById.get(e.seat) : null;
    const slot = (item && L.seatSlot(item)) || L.standSlot(e.level || 0, e.x, e.z);
    slots.set(key, slot);
  }
  return { slots, plan };
}
