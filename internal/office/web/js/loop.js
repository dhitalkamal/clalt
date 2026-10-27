// the page loop: draws only when the scheduler says a frame is due, and sleeps on a timer in between
import { stage, render, app } from './app.js';
import { step } from './step.js';
import { updateHover } from './input.js';
import { createSched } from './sched.js';
import { agentsMoving } from './motion.js';
import { createHud } from './perfhud.js';

const camKey = () => {
  const c = stage.camera;
  const e = c.matrixWorld.elements;
  return [e[12], e[13], e[14], e[0], e[2], e[4], e[6], e[8], e[10], c.fov, c.zoom];
};
const same = (a, b) => a && b && a.every((v, i) => Math.abs(v - b[i]) < 1e-6);

export function startLoop() {
  const sched = createSched(performance.now());
  const hud = createHud(stage.renderer);
  let last = performance.now();
  let cam = null;
  let timer = 0;
  let raf = 0;

  const arm = () => {
    if (timer || raf) return;
    const wait = sched.waitMs(performance.now());
    if (wait <= 4) raf = requestAnimationFrame(frame);
    else timer = setTimeout(() => { timer = 0; raf = requestAnimationFrame(frame); }, wait);
  };
  const wake = () => {
    sched.wake();
    if (timer) { clearTimeout(timer); timer = 0; }
    arm();
  };
  const poke = (kind) => { sched.input(performance.now(), kind); wake(); };

  function frame(now) {
    raf = 0;
    try {
      if (sched.due(now)) {
        const t0 = performance.now();
        const dt = Math.max(0, Math.min((now - last) / 1000, sched.dtCap(now)));
        last = now;
        step(dt);
        updateHover();
        render();
        sched.ran(now);
        sched.setAgents(agentsMoving(app.agents.values()) || !!app.editing);
        const k = camKey();
        if (!same(k, cam)) sched.cameraMoved(now);
        cam = k;
        hud.frame(now, performance.now() - t0, sched.mode(now));
      }
    } catch (e) {
      console.error(e);
    } finally {
      arm();
    }
  }

  stage.wake = wake;
  const on = (t, fn) => addEventListener(t, fn, { capture: true, passive: true });
  on('pointerdown', () => { sched.setDragging(true); poke('drag'); });
  const up = () => { sched.setDragging(false); poke('move'); };
  on('pointerup', up);
  on('pointercancel', up);
  on('pointermove', (e) => poke(e.buttons ? 'drag' : 'move'));
  on('wheel', () => poke('wheel'));
  on('keydown', (e) => {
    if (e.key === 'F' && e.shiftKey && !/INPUT|TEXTAREA|SELECT/.test(e.target.tagName || '')) { hud.toggle(); wake(); return; }
    poke('key');
  });
  on('keyup', () => poke('key'));
  on('resize', wake);
  document.addEventListener('visibilitychange', () => { if (!document.hidden) poke('move'); });
  arm();
}
