// frame scheduler: pure state machine that says how often the page should draw. no dom, no three.js.
//   active   60 fps: dragging, wheel, keys, or the camera moved a moment ago
//   agents   15 fps: agents move or type, or the pointer is roaming over the page
//   ambient   2 fps: nothing moves, a heartbeat for the water and other tiny animations
//   idle      same rate as ambient: no input for two minutes
export const HOLD_MS = 400;
export const IDLE_MS = 120000;
// ms between frames. active is 0: every display frame
export const INTERVAL = { active: 0, agents: 1000 / 15, ambient: 500, idle: 500 };
// biggest time step one frame may simulate, seconds. slow modes need a bigger one to keep animations on time
const DT_CAP = { active: 0.1, agents: 0.1, ambient: 0.6, idle: 0.6 };

export function createSched(now0 = 0) {
  let lastInput = now0;
  let lastActive = -Infinity;
  let lastMove = -Infinity;
  let lastFrame = -Infinity;
  let agents = false;
  let dragging = false;
  let forced = true;

  const mode = (now) => {
    if (dragging || now - lastActive < HOLD_MS) return 'active';
    if (now - lastInput >= IDLE_MS) return 'idle';
    if (agents || now - lastMove < HOLD_MS) return 'agents';
    return 'ambient';
  };
  const waitMs = (now) => (forced ? 0 : Math.max(0, lastFrame + INTERVAL[mode(now)] - now));

  return {
    mode,
    waitMs,
    due: (now) => waitMs(now) <= 0,
    dtCap: (now) => DT_CAP[mode(now)],
    ran(now) { lastFrame = now; forced = false; },
    // user input: drag, wheel and key need 60 fps, a plain move only wakes the page
    input(now, kind) {
      lastInput = now;
      forced = true;
      if (kind === 'move') lastMove = now;
      else lastActive = now;
    },
    // the camera changed on its own (tween, inertia, walking)
    cameraMoved(now) { lastActive = now; },
    // something changed that needs a redraw soon but is not input (snapshot, layout)
    wake() { forced = true; },
    setAgents(on) { agents = !!on; },
    setDragging(on) { dragging = !!on; },
  };
}
