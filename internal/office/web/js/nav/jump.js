// the jump: height h above the floor and vertical speed vy. pure, no three.js.
export const JUMP_H = 0.6;
export const GRAVITY = 9;
const JUMP_V = Math.sqrt(2 * GRAVITY * JUMP_H);

// on the floor (not mid-air) and not on the stairs
export const canJump = (j, onStairs) => j.h === 0 && j.vy === 0 && !onStairs;

export function startJump(j, onStairs = false) {
  if (!canJump(j, onStairs)) return false;
  j.vy = JUMP_V;
  return true;
}

// advances the arc by dt. ceil caps the height (a deck overhead). landed is true on the step that touches down
export function stepJump(j, dt, ceil = Infinity) {
  if (j.h === 0 && j.vy === 0) return { landed: false };
  j.h += j.vy * dt - 0.5 * GRAVITY * dt * dt;
  j.vy -= GRAVITY * dt;
  if (j.h > ceil) { j.h = ceil; if (j.vy > 0) j.vy = 0; }
  if (j.h <= 0) { j.h = 0; j.vy = 0; return { landed: true }; }
  return { landed: false };
}
