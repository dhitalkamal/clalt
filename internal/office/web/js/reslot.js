// one agent after a seat re-plan, for the hall and the garden. carryAction (garden/carry.js) decides: the same seat
// keeps its object (re-pointed at a rebuilt layout), a moved seat or another kind of seat gets a fresh rig and walks
// there from where the old agent stood, an intern past the stools loses its character, an agent the roster no
// longer has is dropped. the scene work comes in through deps, so node tests can drive it with plain objects.
import { carryAction, snapshotMove, walkToSeat, remapAgent } from './garden/carry.js';

export function reslot(L, id, a, ns, { recordOf, removeAgent, makeAgent }) {
  const rec = recordOf(id);
  const act = carryAction(a, ns, rec, L);
  if (act === 'keep') return;
  if (act === 'drop') { removeAgent(id); return; }
  if (act === 'hide') { rec.hid = true; removeAgent(id); return; }
  if (act === 'remap') { remapAgent(a, L, ns); return; }
  const snap = snapshotMove(a);
  removeAgent(id);
  const n = makeAgent(rec, true);
  if (!n) return;
  // the new desk rig grows in while the agent walks over from its old place
  n.scale = 0.001;
  n.rig.scale.setScalar(0.001);
  walkToSeat(n, snap, L);
}
