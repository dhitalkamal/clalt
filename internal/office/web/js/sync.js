// turns a snapshot into the roster the scene shows: new sessions and subagents walk in,
// finished ones walk out, state changes are picked up by the agents on their own.
import { setAccountOrder } from './layout.js';
import { planBreaks } from './planner.js';
import { app, ui, relayout, makeAgent, refreshLabels } from './app.js';
import { MAX_SESSIONS, MAX_SUBS, makeSession, makeSub, modelName, tintFor } from './model.js';

const KNOWN_VERSION = 1;
let lastOrder = null;

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
    const subBy = new Map((snap.subagents || []).map((x) => [s.id + '/' + x.id, x]));
    for (const sub of s.subs) {
      if (sub.leaving) continue;
      const cur = subBy.get(sub.id);
      if (!cur || cur.state === 'done') sub.leaving = true;
      else sub.task = cur.label || sub.task;
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
  setAccountOrder(order);
  order.forEach(tintFor);
  app.accounts = new Map(snap.accounts.map((a) => [a.id, { label: a.label || a.id, limits: a.limits }]));
  ui.accounts = app.accounts;
  updateKnown(snap.sessions);
  const born = [];
  const changed = addNew(snap.sessions, born);
  if (changed || reorder || !app.layout) relayout();
  else refreshLabels();
  for (const rec of born) if (!app.agents.has(rec.id)) makeAgent(rec, first);
  if (first) {
    // idle agents start in the middle of a break so the cafe and play area are in use from the first frame
    for (const a of planBreaks(app, true)) a.instantBreak(a.planOffer, a.planOffer.steps.length - 1);
  }
  ui.loaded = true;
}
