// turns a snapshot into the roster the scene shows: new sessions and subagents walk in,
// finished ones walk out, state changes are picked up by the agents on their own.
import { planBreaks } from './planner.js';
import { app, ui, relayout, makeAgent, refreshLabels } from './app.js';
import { classifySubs, updateMembers, rosterOf } from './roster.js';
import { trackRecord } from './behave/records.js';
import { subIdsOf, newBriefs } from './behave/brief.js';
import { reduced } from './behave/motion.js';
import { MAX_SESSIONS, MAX_SUBS, makeSession, makeSub, modelName, tintFor } from './model.js';

const KNOWN_VERSION = 1;
let lastOrder = null;
let lastRoster = '';

// starting is a session that has not said anything yet, which looks like waiting for input
const stateOf = (s) => (s === 'starting' ? 'idle' : s);

const leave = (s) => { s.leaving = true; s.subs.forEach((x) => { x.leaving = true; }); };

// updates what is already on screen and starts the exit of what the snapshot no longer has
function updateKnown(snapSessions) {
  const by = new Map(snapSessions.map((x) => [x.handle, x]));
  for (const s of app.sessions) {
    if (s.leaving) continue;
    const snap = by.get(s.id);
    if (!snap || snap.state === 'ended') { leave(s); continue; }
    s.state = stateOf(snap.state);
    s.reason = snap.needsYouReason || '';
    s.model = modelName(snap.model);
    s.actions = snap.actions;
    trackRecord(s, snap.doing, Date.now());
    const subBy = new Map((snap.subagents || []).map((x) => [s.id + '/' + x.id, x]));
    for (const sub of s.subs) {
      if (sub.leaving) continue;
      const cur = subBy.get(sub.id);
      if (!cur || cur.state === 'done') sub.leaving = true;
      else { sub.task = cur.label || sub.task; trackRecord(sub, cur.doing, Date.now()); }
    }
  }
}

// adds the subagents the session does not show yet. returns the ones it made.
function addSubs(session, snapSubs) {
  const made = [];
  for (const sub of snapSubs || []) {
    if (sub.state === 'done' || session.subs.some((x) => x.id === session.id + '/' + sub.id)) continue;
    if (session.subs.filter((x) => !x.leaving).length >= MAX_SUBS) break;
    const rec = makeSub(session, app.sessions.flatMap((x) => x.subs), sub);
    trackRecord(rec, sub.doing, Date.now());
    session.subs.push(rec);
    made.push(rec);
  }
  return made;
}

// returns true when the roster changed, so the layout must be rebuilt
function addNew(snapSessions, born) {
  let changed = false;
  app.hidden = 0;
  const known = new Map(app.sessions.map((s) => [s.id, s]));
  for (const snap of snapSessions) {
    let s = known.get(snap.handle);
    if (!s) {
      if (snap.state === 'ended') continue;
      if (app.sessions.filter((x) => !x.leaving).length >= MAX_SESSIONS) { app.hidden++; continue; }
      s = makeSession(snap, snap.account);
      s.state = stateOf(snap.state);
      trackRecord(s, snap.doing, Date.now());
      app.sessions.push(s);
      born.push(s);
      changed = true;
    }
    if (s.leaving) continue;
    const subs = addSubs(s, snap.subagents);
    if (subs.length) { born.push(...subs); changed = true; }
  }
  return changed;
}

export function applySnapshot(snap) {
  if (snap.v > KNOWN_VERSION) { ui.setNotice('this clalt is newer than the page it serves, update clalt'); return; }
  ui.setNotice('');
  const first = !ui.loaded;
  const order = snap.accounts.map((a) => a.id);
  for (const s of snap.sessions) if (!order.includes(s.account)) order.push(s.account);
  const orderKey = order.join(',');
  const reorder = orderKey !== lastOrder;
  lastOrder = orderKey;
  order.forEach(tintFor);
  app.accounts = new Map(snap.accounts.map((a) => [a.id, { label: a.label || a.id, limits: a.limits, forecast: a.forecast }]));
  ui.accounts = app.accounts;
  const before = subIdsOf(app.sessions);
  updateKnown(snap.sessions);
  const born = [];
  let changed = addNew(snap.sessions, born);
  // team: classify runs (mapped onto a member or intern), then update the member records. an older server has no team.
  const team = Array.isArray(snap.team) ? snap.team : [];
  app.team = team; // the renderer reads plate names and strip colours from it on every seat refresh
  classifySubs(app.sessions, new Set(team.map((e) => e.id)));
  const known = new Set(app.members.map((m) => m.id));
  app.members = updateMembers(app.members, team, app.sessions);
  for (const m of app.members) if (!known.has(m.id) && !m.leaving) born.push(m);
  const sig = rosterOf(app.sessions, app.members).map((e) => e.key + ':' + e.kind).join(',');
  if (sig !== lastRoster) { lastRoster = sig; changed = true; }
  if (changed || reorder || !app.layout) relayout();
  else refreshLabels();
  for (const rec of born) if (!rec.mapped && !app.agents.has(rec.id)) makeAgent(rec, first);
  // a main agent walks over to brief each subagent that appeared while the page was running
  if (!reduced()) for (const b of newBriefs(before, subIdsOf(app.sessions), first)) app.agents.get(b.main)?.queueBrief(b.sub);
  if (first) {
    // idle agents start in the middle of a break so the cafe and play area are in use from the first frame
    for (const a of planBreaks(app, true)) a.instantBreak(a.planOffer, a.planOffer.steps.length - 1);
  }
  ui.loaded = true;
}
