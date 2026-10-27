// follow an agent: the camera trails behind it, third person, and looks at its head. pure of three.js, it only
// reads the agent model's world matrix and sets camera.position / lookAt. update returns 'back' once done.
const BACK = 3.5;
const UP = 2.2;
const HEAD = 0.8;
const KEYS = ['KeyW', 'KeyA', 'KeyS', 'KeyD', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Escape'];

// position and facing (x, z unit vector) of an agent from its model's world matrix
function agentSpot(a) {
  const m = a?.model;
  if (!m) return null;
  m.updateWorldMatrix?.(true, false);
  const e = m.matrixWorld.elements;
  const fl = Math.hypot(e[8], e[10]) || 1;
  return { x: e[12], y: e[13], z: e[14], fx: e[8] / fl, fz: e[10] / fl };
}

export function createFollow(agentId) {
  let look = [0, 0, 0];
  const c = {
    name: 'follow',
    done: false,
    enter(ctx, from) {
      this.done = false;
      look = from ? [...from.target] : [0, 0, 0];
      if (from) { const p = ctx.stage.camera.position; p.set(...from.pos); }
    },
    update(ctx, dt, input) {
      const spot = agentSpot(ctx.app.agents.get(agentId));
      if (!spot || KEYS.some((k) => input.keys.has(k))) { this.done = true; return 'back'; }
      const want = [spot.x - spot.fx * BACK, spot.y + UP, spot.z - spot.fz * BACK];
      const gy = ctx.garden?.groundY?.(want[0], want[2]) ?? 0;
      want[1] = Math.max(want[1], gy + 0.4);
      const aim = [spot.x, spot.y + HEAD, spot.z];
      const cam = ctx.stage.camera;
      const k = ctx.reducedMotion ? 1 : 1 - Math.exp(-dt * 4);
      const p = cam.position;
      p.set(p.x + (want[0] - p.x) * k, p.y + (want[1] - p.y) * k, p.z + (want[2] - p.z) * k);
      look = look.map((v, i) => v + (aim[i] - v) * k);
      cam.lookAt(...look);
    },
    exit(ctx) {
      const p = ctx.stage.camera.position;
      return { pos: [p.x, p.y, p.z], target: [...look] };
    },
    setView() {},
    dispose() {},
  };
  return c;
}
