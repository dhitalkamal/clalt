// working agents per room, for the number on a room label. pure.
// slots: Map(agent id -> {room}), states: Map(agent id -> 'busy'|'idle'|...)
export function workingByRoom(slots, states) {
  const out = new Map();
  for (const [id, slot] of slots) {
    if (!slot.room || states.get(id) !== 'busy') continue;
    out.set(slot.room, (out.get(slot.room) || 0) + 1);
  }
  return out;
}
