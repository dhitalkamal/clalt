// shared sizes of the office scene. the floor plan itself comes from a layout json, see scene/plan.js.
export const G = 0.5; // world units per layout cell
export const MEZ = 1.29; // deck height, matches the wall tile seam
export const SEAT_OFF = 0.36; // chair centre behind the desk centre (north of it)

// the room type a subagent type works in. the seat planner may overflow it to the workspace.
export function spaceFor(type) {
  if (type === 'Explore') return 'research';
  if (type === 'Plan') return 'meeting';
  if (/review|test|hunter|analy/i.test(type)) return 'review';
  return 'workspace';
}
