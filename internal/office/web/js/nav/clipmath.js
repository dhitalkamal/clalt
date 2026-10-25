// which Kenney clip the avatar plays and how fast. the clips are the ones the agents use (idle, walk) plus
// sprint, jump and fall from the same files. pure, no three.js.
const WALK_REF = 1.2; // ground speed the walk clip looks right at
const SPRINT_REF = 3; // and the sprint clip
const CAP = [0.6, 1.9];
const clamp = (v) => Math.min(CAP[1], Math.max(CAP[0], v));

export function clipFor({ speed, running, air, vy }) {
  if (air) return { name: vy > 0 ? 'jump' : 'fall', timeScale: 1 };
  if (speed < 0.15) return { name: 'idle', timeScale: 1 };
  if (running) return { name: 'sprint', timeScale: clamp(speed / SPRINT_REF) };
  return { name: 'walk', timeScale: clamp(speed / WALK_REF) };
}
