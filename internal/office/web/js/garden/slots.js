// who sits where in a garden: routing.planSeats plans on the seats of the renderer (routing/garden.js),
// then plan entries turn into the slots the agents walk to.
import * as routing from '../routing.js';
import { rosterOf } from '../roster.js';
import { gardenSeats, gardenOverflow } from './seats.js';

let registered = false;

// the source hands the planner the seats and overflow spots of a layout
function register() {
  if (registered) return;
  routing.setGardenSource((layout) => ({ seats: gardenSeats(layout), overflow: gardenOverflow(layout) }));
  registered = true;
}

export { rosterOf };

// returns the slots by agent key and the raw plan (kept for the next call so seats stay put)
export function assignGardenSeats(L, sessions, catalog, prev, members = []) {
  register();
  const roster = rosterOf(sessions, members);
  const plan = routing.planSeats(L.layout, roster, catalog, prev);
  const slots = new Map();
  for (const [key, e] of plan) {
    if (e.kind === 'hidden') continue; // an intern past the stools: no slot, only a badge
    const seat = e.kind === 'seat' ? L.seatById.get(e.seat) : null;
    slots.set(key, seat ? L.seatSlot(seat) : L.standSlot(e.level || 0, e.x, e.z, e.rot || 0));
  }
  // hidden interns have no slot, the badge counts say how many sit out per team id
  return { slots, plan, badges: routing.badgeCounts(plan) };
}
