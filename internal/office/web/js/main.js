import { loadAll } from './assets.js';
import { ui, resize, render } from './app.js';
import { step } from './step.js';
import { bindInput, updateHover } from './input.js';
import { startPoll } from './poll.js';
import { applySnapshot } from './sync.js';
import { loadFirst } from './layoutload.js';
import { applyLayout } from './scene/apply.js';
import { startView } from './viewctl.js';
import { installActions } from './actions/index.js';
import { initSummary } from './summary/index.js';

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
  const first = await loadFirst();
  await applyLayout(first.layout);
  loadingEl.hidden = true;
  bindInput();
  installActions();
  initSummary();
  resize();
  addEventListener('resize', resize);
  const view = startView(first);
  startPoll((snap) => { try { applySnapshot(snap); view.reviews(snap.reviews); } catch (e) { console.error(e); ui.setNotice('the page could not read the snapshot'); } }, onError, () => view.check());
  loop();
}

boot().catch((e) => { console.error(e); loadingEl.textContent = 'load failed: ' + e.message; });
