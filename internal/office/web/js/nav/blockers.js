// points the ground spawn keeps clear of: every agent's spot and every seat. the hall passes its ctx, the garden
// reads its seats off the layout json. a failing read gives fewer points, never an error.
import { gardenSeats } from '../garden/seats.js';

export function blockers(app, hall) {
  const out = [];
  for (const a of app.agents ? app.agents.values() : []) {
    const m = a && a.model;
    if (!m) continue;
    if (m.updateWorldMatrix) m.updateWorldMatrix(true, false);
    const e = m.matrixWorld && m.matrixWorld.elements;
    if (e) out.push({ x: e[12], z: e[14] });
  }
  try {
    const seats = hall ? hall.seats() : app.layoutJson && app.layoutJson.v === 2 ? gardenSeats(app.layoutJson) : [];
    for (const s of seats) out.push({ x: s.x, z: s.z });
  } catch (e) { /* seats unknown, agents still count */ }
  return out;
}
