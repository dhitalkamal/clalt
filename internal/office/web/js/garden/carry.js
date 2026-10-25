// keeps agents alive across a campus rebuild. the agent objects stay; only what points into the old walk graph
// is re-planned (paths, break spots, nodes). pure, no three.js, so node tests can drive it with plain objects.
import { STAND_YAW } from '../agentConst.js';

const DECK_Y = 0.65;

export const levelOf = (y) => (y > DECK_Y ? 1 : 0);

// a throwaway node where the agent stands now, so the router starts from the real position
export const tempNode = (L, pos) => L.node(levelOf(pos.y), { x: pos.x, z: pos.z }, []);

// the walk from a position to a goal node on the graph of L, starting exactly at pos
export function replanPath(L, pos, dest) {
  const pts = L.path(tempNode(L, pos), dest);
  return [{ x: pos.x, z: pos.z, y: pos.y }, ...pts.slice(1)];
}

// another seat, another kind of place, or a stand spot that moved
export function slotMoved(a, b) {
  if (!a || !b || a === b) return false;
  if (a.itemId !== b.itemId || a.kind !== b.kind || a.side !== b.side) return true;
  return !a.itemId && (Math.hypot(a.x - b.x, a.z - b.z) > 0.05 || a.level !== b.level);
}

// where the character is in the world (a seated one rides in the seat group, so use the chair)
function worldPos(a) {
  if (a.mode === 'seated') return { x: a.slot.chair.x, y: a.slot.y, z: a.slot.chair.z };
  const p = a.model.position;
  return { x: p.x, y: p.y, z: p.z };
}

const KEEP = ['cupHeld', 'cupHotUntil', 'cupHideAt', 'nextBreakAt', 'breakCount', 'carryPaper'];

// the movement state worth moving to a new agent object
export function snapshotMove(a) {
  const snap = { mode: a.mode, pos: worldPos(a), yaw: a.model.rotation.y };
  for (const k of KEEP) snap[k] = a[k];
  return snap;
}

export function restoreMove(a, snap) {
  a.mode = snap.mode;
  Object.assign(a.model.position, snap.pos);
  a.model.rotation.y = snap.yaw;
  for (const k of KEEP) a[k] = snap[k];
}

// a (re)built agent walks from where the old one stood to its seat on the new graph, then sits
export function walkToSeat(a, snap, L) {
  const pos = snap.pos;
  for (const k of KEEP) a[k] = snap[k];
  a.plan = null;
  a.planOffer = null;
  a.detach();
  a.place(pos, snap.yaw);
  const pts = replanPath(L, pos, a.slot.node);
  a.follow('back', () => pts, () => a.arrive(), (_L, ag) => ag.slot.node);
  a.curNode = a.slot.node;
}

// re-points a kept agent at the new layout: slot, nodes, break spot and the path it was walking
export function remapAgent(a, L, slot) {
  a.slot = slot;
  a.L = L;
  // what is bound to a seat moves with the new scene at once: the desk rig, a standing agent, an agent at a break spot
  if (a.rig) { a.rig.position.set(slot.x, slot.y, slot.z); a.rig.rotation.y = slot.rotY; }
  if (a.mode === 'seated' || a.mode === 'standing') a.curNode = slot.node;
  if (a.mode === 'standing') {
    const last = slot.stand[slot.stand.length - 1];
    a.place({ x: last.x, y: slot.y, z: last.z }, slot.rotY + STAND_YAW);
  }
  if (a.mode === 'leisure') {
    const spot = a.spot ? L.spots[a.spot.name] : null;
    if (spot) {
      a.spot = spot;
      a.curNode = spot.node;
      a.model.position.x = spot.x;
      a.model.position.z = spot.z;
      a.model.rotation.y = spot.yaw;
    } else if (a.spot) {
      // the spot is gone: end the break and walk back from where the agent stands
      a.spot = null;
      a.curSpot = null;
      a.abort = true;
      a.curNode = tempNode(L, worldPos(a));
    } else if (a.act === 'handoff') a.curNode = tempNode(L, worldPos(a));
  }
  const p = a.path;
  if (a.mode === 'walking' && p && p.dest) {
    const goal = p.dest(L, a);
    if (goal) {
      const pts = replanPath(L, worldPos(a), goal);
      p.fn = () => pts;
      p.wp = 0;
    }
  }
}
