import { loadAll } from './assets.js';
import { ui, resize, render } from './app.js';
import { step } from './step.js';
import { bindInput, updateHover } from './input.js';
import { startPoll } from './poll.js';
import { applySnapshot } from './sync.js';
import { loadLayout } from './layoutload.js';
import { applyLayout } from './scene/apply.js';
import { startEditor } from './editorhook.js';

const loadingEl = document.getElementById('loading');

function loop() {
  let last = performance.now();
  const frame = (now) => {
    // schedule first, so one bad frame cannot stop the animation for good
    requestAnimationFrame(frame);
    const dt = Math.max(0, Math.min((now - last) / 1000, 0.05));
    last = now;
    try {
      step(dt);
      updateHover();
      render();
    } catch (e) {
      console.error(e);
    }
  };
  requestAnimationFrame(frame);
}

// the poll tells the page when it cannot reach clalt, and says what to do about it
function onError(e) {
  ui.setNotice(e.message === 'auth'
    ? 'this page lost its access. restart clalt office and open the new link'
    : 'lost the connection to clalt office, trying again');
}

async function boot() {
  const bar = document.getElementById('loadbar');
  await loadAll((p) => { bar.style.width = p * 100 + '%'; });
  const { layout, edit } = await loadLayout();
  await applyLayout(layout);
  loadingEl.hidden = true;
  bindInput();
  resize();
  addEventListener('resize', resize);
  startPoll((snap) => { try { applySnapshot(snap); } catch (e) { console.error(e); ui.setNotice('the page could not read the snapshot'); } }, onError);
  loop();
  // the editor exists only when clalt office runs with --edit
  if (edit) startEditor(layout).catch((e) => { console.error(e); ui.setNotice('the editor could not start: ' + e.message); });
}

boot().catch((e) => { console.error(e); loadingEl.textContent = 'load failed: ' + e.message; });
