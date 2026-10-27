// session records the scene works on. sync.js makes them from snapshots, the scene reads them.
import { spaceFor } from './layout.js';

export const MAX_SESSIONS = 20;
export const MAX_SUBS = 8;

const PALETTE = [0x4f86c6, 0x58b08a, 0xd98a3d, 0xa67bc9, 0xc9677b, 0x5fb3b3, 0xb5a23c, 0x7a8aa8];
export const ACCOUNT_TINT = {};

// the first time an account is seen it gets the next colour, and keeps it for the page's life
export function tintFor(account) {
  if (ACCOUNT_TINT[account] === undefined) ACCOUNT_TINT[account] = PALETTE[Object.keys(ACCOUNT_TINT).length % PALETTE.length];
  return ACCOUNT_TINT[account];
}

// small stable hash, so a session looks and behaves the same after a reload
export function hash(str) {
  let h = 2166136261;
  for (let i = 0; i < str.length; i++) h = Math.imul(h ^ str.charCodeAt(i), 16777619);
  return (h >>> 0) % 100000;
}

// claude-opus-5-5[1m] -> Opus 5.5, anything else is shown as it is
export function modelName(id) {
  if (!id) return 'unknown';
  const m = /^(?:claude-)?(opus|sonnet|haiku)-(\d+(?:-\d+)?)/i.exec(id.replace(/\[.*?\]/g, ''));
  if (!m) return id;
  return m[1][0].toUpperCase() + m[1].slice(1).toLowerCase() + ' ' + m[2].replace('-', '.');
}

export function makeSession(snap, account) {
  return {
    id: snap.handle, role: 'main', seed: hash(snap.handle), account, leaving: false, subs: [],
    state: snap.state, reason: snap.needsYouReason || '', handle: snap.handle, project: snap.project,
    model: modelName(snap.model), actions: snap.actions, startedAt: Date.parse(snap.startedAt) || 0,
  };
}

// space starts as the room type the sub's type prefers, the seat planner sets the room it really got
export function makeSub(session, allSubs, snap) {
  const type = snap.type || 'agent';
  const id = session.id + '/' + snap.id;
  return { id, role: 'sub', seed: hash(id), account: session.account, state: 'busy', leaving: false,
    session, type, task: snap.label || '', space: spaceFor(type), startedAt: Date.parse(snap.startedAt) || 0 };
}
