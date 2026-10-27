// what the page offers for a session, from the snapshot's per session "actions". pure.
export const OUTSIDE = 'started outside clalt: use clalt run to control it';
const NO_ANSWER = 'this clalt cannot control sessions, update it';
const GENERIC = 'not available for this session';

// returns {move, focus, stop}, each {enabled, why}: why is empty when enabled
export function actionView(actions) {
  const out = {};
  for (const k of ['move', 'focus', 'stop']) {
    if (!actions || typeof actions !== 'object') { out[k] = { enabled: false, why: NO_ANSWER }; continue; }
    const on = actions[k] === true;
    out[k] = { enabled: on, why: on ? '' : (typeof actions.why === 'string' && actions.why) || GENERIC };
  }
  return out;
}
