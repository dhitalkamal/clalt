// the v2 renderer: a garden campus built from a layout v2 json. scene/apply.js sends v2 layouts here and v1
// layouts to the hall code, unchanged.
//
// seams (all names are exported from this file):
//   applyGarden(layout)       builds the scene, the walk graph and the agents' seats. a template switch calls it
//                             again: the old scene is freed first (renderer.info.memory returns to its base).
//   teardownGarden()          frees the garden and gives the stage back to the hall. apply.js calls it for v1.
//   gardenSeats(layout)       [{id, pavilionId, type, account, y, x, z, rot, kind, ...}] in world units, rot in
//                             degrees, kind 'desk'|'cafe'|'play'|'review'|'meeting'|'lobby'|'stool'. a team
//                             pavilion gives each member a desk seat (member id) and 3 'stool' seats (member, index). desks are the desk
//                             centre, the chair is 0.36 behind it, round seats are the chair centre.
//   gardenOverflow(layout)    [{id, x, z, y, rot, near}] standing spots for agents without a seat.
//   gardenWalk(layout)        walk graph: paths, bridges, pavilion floors and the treehouse stairs are walkable,
//                             water, trees, furniture and walls are not. {path(a, b), node, nodeForSeat,
//                             walkable(x, z, level), reachable(node), spawn}. points are {x, z, y}.
//   setMemberState(agent, 'idle'|'busy')  dims a team member (grey tint, less colour, monitor off) or lights it
//                             (own colours, monitor on, glow ring). agent is the Agent object. the page applies
//                             it by itself whenever an agent record with kind 'team' changes state, so the routing
//                             side only needs it for a state the record does not carry. works on hall agents too.
//   makeInternLook(agent, color)  0.8 scale, cap and lanyard in the colour. agent.js calls it for records with
//                             kind 'intern' (record.color is the member colour, '#rrggbb').
//   setDeskBadge(memberId, n) the +N pill over a team desk, 0 hides it. drawn in the label layer (css .badge).
//                             also read from app.badges (Map teamId -> count) on every seat refresh.
//   setGardenTeam(list)       the snapshot's team list: names on the nameplates, colours on the desk strips. also
//                             read from app.team on every seat refresh when the sync side stores it there.
//   cellToWorld, worldToCell  identity for a garden, positions are world units already.
// the agent system sees the same names as the hall: app.layout.slots, seatSlot, standSlot, spots, groups, path and
// spawn (see compat.js). seat plans come from routing.planSeats when routing.js offers setGardenSource, else
// from the fallback planner in assign.js.
import * as THREE from 'three';
import { findRecord, agentRecords } from '../roster.js';
import { app, world, stage, ui, drop, makeAgent, removeAgent } from '../app.js';
import { applyView, refreshShadows } from '../stage.js';
import { tintFor } from '../model.js';
import { compileGarden } from './compat.js';
import { buildGarden } from './build.js';
import { disposeRes } from './res.js';
import { enterGardenStage, leaveGardenStage, fitLight, fitFog } from './stagefx.js';
import { goTo, cameraTick, presetPose, fitBounds } from './cameras.js';
import { assignGardenSeats } from './slots.js';
import { carryAction, snapshotMove, walkToSeat, remapAgent } from './carry.js';
import { buildSpares } from './dynamic.js';
import { gardenLabels } from './labels.js';
import { onBadgeChange, syncBadges } from './badge.js';
import { updateTeam } from './teamplates.js';

export { gardenSeats, gardenOverflow } from './seats.js';
export { gardenWalk } from './walk.js';
export { setMemberState, makeInternLook } from './teamlook.js';
export { setDeskBadge } from './badge.js';
export { updateTeam as setGardenTeam } from './teamplates.js';
export { PRESET_IDS, goTo, setSpin, setSafe, currentId } from './cameras.js';

export const cellToWorld = (level, x, z) => ({ x, y: level ? 1.29 : 0, z });
export const worldToCell = (level, p) => ({ x: p.x, z: p.z });

let state = null;

export const isGarden = () => state !== null;

const recordOf = (key) => findRecord(app.sessions, app.members, key);

// the room a subagent works in, for the hover card
function spaceOf(L, slot) {
  if (slot.seatKind === 'review' || slot.seatKind === 'meeting') return slot.seatKind;
  const pav = (L.layout.pavilions || []).find((p) => p.id === slot.room);
  return pav ? pav.type : 'lobby';
}

// one agent after a re-plan. carryAction (pure, carry.js) decides: same seat keeps its object (re-pointed at a rebuilt
// layout), a moved seat or another kind of seat gets a fresh rig and walks there from where the old agent stood, an
// intern past the stools loses its character (counted in a badge), an agent the roster no longer has is dropped
function reslot(L, id, a, ns) {
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
  n.scale = 0.001;
  n.rig.scale.setScalar(0.001);
  walkToSeat(n, snap, L);
}

// a hidden intern gets its character back when a stool frees up
function unhide(slots) {
  for (const rec of agentRecords(app.sessions, app.members)) {
    if (rec.hid && slots.has(rec.id) && !app.agents.has(rec.id)) makeAgent(rec, false);
  }
}

// roster changed: re-plan seats, redraw the spare furniture, re-create agents whose rig no longer fits
export function refreshGardenSeats() {
  const L = app.layout;
  if (!state || !L || L.v !== 2) return;
  if (Array.isArray(app.team)) updateTeam(app.team);
  const { slots, plan, badges } = assignGardenSeats(L, app.sessions, app.catalog, app.seatPlan, app.members);
  app.seatPlan = plan;
  app.badges = badges;
  syncBadges(badges);
  L.slots = slots;
  const taken = new Set();
  for (const [key, slot] of slots) {
    const rec = recordOf(key);
    if (rec && rec.role === 'sub') rec.space = spaceOf(L, slot);
    if (slot.itemId) taken.add(slot.itemId);
  }
  drop(app.dynamic && app.dynamic.group);
  app.dynamic = { group: buildSpares(L, taken, tintFor), sway: [], wings: [] };
  world.add(app.dynamic.group);
  for (const [id, a] of [...app.agents]) reslot(L, id, a, slots.get(id));
  unhide(slots);
  ui.setLabels(gardenLabels(L, app.accounts));
  refreshShadows(stage);
}

// the page camera follows the rig: a pose is applied by mutating stage.view in place, so a fresh stage.view object
// means something else (the Esc key, the hall buttons) reset the view and the rig has to follow
function makeRig(bb, eye, old) {
  const v = stage.view || { dist: 40, target: new THREE.Vector3(), az: 0, el: 1 };
  stage.view = v;
  const rig = {
    bb, eye, safe: old ? old.safe : {}, id: old ? old.id : 'top', tween: null, spin: old ? old.spin : false, viewRef: v,
    aspect: () => innerWidth / Math.max(1, innerHeight),
    view: () => ({ az: v.az, el: v.el, dist: v.dist, target: { x: v.target.x, y: v.target.y, z: v.target.z }, fov: stage.camera.fov }),
    apply(pose) {
      v.az = pose.az; v.el = pose.el; v.dist = pose.dist;
      v.target.set(pose.target.x, pose.target.y, pose.target.z);
      if (stage.camera.fov !== pose.fov) { stage.camera.fov = pose.fov; stage.camera.updateProjectionMatrix(); }
      fitFog(stage, pose.dist);
      applyView(stage);
    },
  };
  stage.rig = rig;
  return rig;
}

// zoom limits, the hall buttons and the sun follow the top view of the new island
function fitStage(bb) {
  const aspect = innerWidth / Math.max(1, innerHeight);
  const top = presetPose('top', { bb, aspect, safe: stage.rig.safe });
  stage.fitDist = { top: top.dist, front: fitBounds(bb, { az: 0, el: 32, fov: 30, aspect, safe: stage.rig.safe }).dist };
  stage.base = { dist: top.dist, target: new THREE.Vector3(top.target.x, top.target.y, top.target.z) };
  stage.bounds = { x: (bb.x1 - bb.x0) / 2, z: (bb.z1 - bb.z0) / 2 };
  stage.vel = { az: 0, el: 0 };
  stage.preset = 'top';
  fitLight(stage, bb, top.dist);
}

function onResize() {
  if (!state) return;
  // the nav manager keeps a free camera where the user left it and refits an untouched preset, the old rig refits
  if (!stage.nav) goTo(stage, stage.rig.id, { instant: true });
  else if (stage.nav.refit) stage.nav.refit();
  fitStage(state.bb);
}

// keep: a rebuild of the campus leaves the agents (and the garden stage settings) alone
function freeGarden(keep) {
  if (!state) return;
  if (!keep) for (const id of [...app.agents.keys()]) removeAgent(id);
  drop(app.dynamic && app.dynamic.group);
  world.remove(state.group);
  disposeRes();
  stage.rig = null;
  if (!keep) leaveGardenStage(stage);
  Object.assign(app, { layout: null, layoutJson: null, dynamic: null, statics: null, shell: null, life: null, ticks: [], seatPlan: null,
    relayoutImpl: null, labelsImpl: null, resizeImpl: null, dimmed: null, printers: [], pongs: [] });
  ui.setLabels([]);
  onBadgeChange(null);
  state = null;
}

export const teardownGarden = () => freeGarden(false);

// builds the garden for a v2 layout and seats the agents that are already on screen
export async function applyGarden(layout) {
  const old = stage.rig;
  // a rebuilt campus keeps the seat plan: agents whose seat still exists sit down where they were
  const plan = app.seatPlan;
  freeGarden(true);
  enterGardenStage(stage);
  const L = compileGarden(layout);
  const built = buildGarden(layout, L.walk.site, L.walk.plan);
  world.add(built.group);
  state = { group: built.group, bb: built.bb, L };
  Object.assign(app, { layout: L, layoutJson: layout, catalog: null, statics: null, shell: null, life: null, seatPlan: plan, dimmed: null, printers: [], pongs: [] });
  let last = null;
  app.ticks = [...built.ticks, (t) => {
    const dt = last === null ? 0 : Math.min(0.1, Math.max(0, t - last));
    last = t;
    const rig = stage.rig;
    // something else replaced stage.view (resetZoom, the hall buttons): drop the tween and the preset fov
    if (rig && stage.view !== rig.viewRef) {
      rig.viewRef = stage.view;
      rig.tween = null;
      rig.id = 'top';
      if (stage.camera.fov !== 30) { stage.camera.fov = 30; stage.camera.updateProjectionMatrix(); }
    }
    cameraTick(stage, dt, app.dragging);
  }];
  makeRig(built.bb, layout.eye, old);
  fitStage(built.bb);
  goTo(stage, stage.rig.id, { instant: true });
  app.relayoutImpl = refreshGardenSeats;
  app.labelsImpl = () => ui.setLabels(gardenLabels(L, app.accounts));
  app.resizeImpl = onResize;
  onBadgeChange(() => ui.setLabels(gardenLabels(L, app.accounts)));
  refreshGardenSeats();
  // agents that survived the rebuild were handled by refreshGardenSeats; the rest are what the page already had.
  // an intern without a stool gets no character (makeAgent marks it hidden)
  for (const rec of agentRecords(app.sessions, app.members)) if (!app.agents.has(rec.id)) makeAgent(rec, true);
  refreshShadows(stage);
}

