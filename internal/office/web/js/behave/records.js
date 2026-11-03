// keeps the doing of a snapshot on the records the scene reads. pure.
import { track } from './work.js';

// rec.track is what the agent acts out, see work.js. now is in milliseconds
export function trackRecord(rec, doing, now) {
  rec.doing = doing || null;
  rec.track = track(rec.track, rec.doing, now);
}

// a team member stands for its runs: it acts out the first one that has work.
// runs are [session, sub] pairs as roster.js makes them
export function memberDoing(runs) {
  for (const [, x] of runs) if (x.doing) return x.doing;
  return null;
}
