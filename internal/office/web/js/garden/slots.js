// who sits where in a garden: routing.planSeats plans on the seats of the renderer (routing/garden.js),
// then plan entries turn into the slots the agents walk to.
import * as routing from '../routing.js';
import { gardenSeats, gardenOverflow } from './seats.js';

let registered = false;

// the source hands the planner the seats and overflow spots of a layout
function register() {
  if (registered) return;
  routing.setGardenSource((layout) => ({ seats: gardenSeats(layout), overflow: gardenOverflow(layout) }));
  registered = true;
}

export const rosterOf = (sessions) => sessions.flatMap((s) => [
  { key: s.id, kind: 'main', account: s.account, type: 'main', state: s.state },
  ...s.subs.map((x) => ({ key: x.id, kind: 'sub', account: s.account, type: x.type, state: 'busy' })),
]);

// returns the slots by agent key and the raw plan (kept for the next call so seats stay put)
export function assignGardenSeats(L, sessions, catalog, prev) {
  register();
  const roster = rosterOf(sessions);
  const plan = routing.planSeats(L.layout, roster, catalog, prev);
  const slots = new Map();
  for (const [key, e] of plan) {
    const seat = e.kind === 'seat' ? L.seatById.get(e.seat) : null;
    slots.set(key, seat ? L.seatSlot(seat) : L.standSlot(e.level || 0, e.x, e.z, e.rot || 0));
  }
  return { slots, plan };
}
