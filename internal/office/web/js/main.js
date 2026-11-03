import { loadAll } from './assets.js';
import { ui, resize } from './app.js';
import { bindInput } from './input.js';
import { startLoop } from './loop.js';
import { startPoll } from './poll.js';
import { applySnapshot } from './sync.js';
import { loadFirst } from './layoutload.js';
import { applyLayout } from './scene/apply.js';
import { startView } from './viewctl.js';
import { installActions } from './actions/index.js';
import { initSummary } from './summary/index.js';
import { bindDock } from './dock.js';
import { pollErrorText } from './lostaccess.js';

const loadingEl = document.getElementById('loading');

// the poll tells the page when it cannot reach clalt, and says what to do about it
function onError(e) {
  ui.setNotice(pollErrorText(e));
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
  bindDock();
  resize();
  addEventListener('resize', resize);
  const view = startView(first);
  startPoll((snap) => { try { applySnapshot(snap); view.reviews(snap.reviews); } catch (e) { console.error(e); ui.setNotice('the page could not read the snapshot'); } }, onError, () => view.check());
  startLoop();
}

boot().catch((e) => { console.error(e); loadingEl.textContent = 'load failed: ' + e.message; });
