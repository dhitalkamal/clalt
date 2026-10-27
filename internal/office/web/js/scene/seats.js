// who sits where: asks the routing module for a plan, then turns it into the slots the agents walk to
import { planSeats } from '../routing.js';
import { rosterOf } from '../roster.js';


// returns the slots by agent key and the raw plan (kept for the next call so seats stay put)
export function assignSeats(L, sessions, catalog, prev, members = []) {
  const plan = planSeats(L.layout, rosterOf(sessions, members), catalog, prev);
  const slots = new Map();
  for (const [key, e] of plan) {
    const item = e.kind === 'seat' ? L.itemById.get(e.seat) : null;
    const slot = (item && L.seatSlot(item)) || L.standSlot(e.level || 0, e.x, e.z);
    slots.set(key, slot);
  }
  return { slots, plan };
}
