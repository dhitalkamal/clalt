// the activity drawer: header, timeline, load older, auto refresh. the state logic is in pager.js,
// the rows in rows.js and render.js. oldest entry on top, newest at the bottom like a chat, so the
// view starts at what just happened and "load older" sits where the older text would be.
import { el } from '../dom.js';
import { buildRows } from './rows.js';
import { fetchActivity as defaultFetch, errorText } from './api.js';
import { renderRows } from './render.js';
import { activityTarget, describeTarget } from './target.js';
import { fetchNewer } from './catchup.js';
import { createActionBar } from '../actions/bar.js';
import { stampNode, chipNode } from '../notebook.js';
import { newPager, pagerKey, matches, applyFirst, applyNewer, applyOlder, startOlder, failOlder, canLoadOlder, shouldRefresh, activityPath } from './pager.js';

const PAGE = 100;

const EVERY = 3000;
const NEAR = 48;

// h: { ctx() -> {accounts, sessions}, fallback: element to focus when the opener is gone, onClose: optional, fetch: optional, quick: optional idle row {el, update(rec)} }
export function createActivityPanel(h) {
  const newView = () => ({ expanded: new Set(), opened: new Set(), openRun: (a, t) => openRun(a, t) });
  const fetchIt = h.fetch || defaultFetch;
  const root = el('aside', 'activity');
  root.setAttribute('role', 'dialog');
  root.setAttribute('aria-modal', 'false');
  root.setAttribute('aria-labelledby', 'activity-title');
  root.hidden = true;
  const title = el('h2', '', '');
  title.id = 'activity-title';
  title.tabIndex = -1;
  const backBtn = el('button', 'back', 'Back');
  backBtn.type = 'button';
  backBtn.hidden = true;
  const closeBtn = el('button', 'close', 'Close');
  closeBtn.type = 'button';
  closeBtn.setAttribute('aria-label', 'Close activity panel');
  const meta = el('dl', 'info');
  const note = el('p', 'quiet');
  const err = el('div', 'err');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const errText = el('span');
  const retry = el('button', '', 'Retry');
  retry.type = 'button';
  err.append(errText, retry);
  const log = el('div', 'log');
  log.tabIndex = 0;
  log.setAttribute('role', 'region');
  log.setAttribute('aria-label', 'activity timeline');
  const fresh = el('button', 'fresh', 'New activity below');
  fresh.type = 'button';
  fresh.hidden = true;
  const bar = createActionBar({ ctx: h.ctx });
  const quick = h.quick || null;
  root.append(el('header', '', backBtn, title, closeBtn), ...(quick ? [quick.el] : []), bar.el, meta, note, err, log, fresh);
  document.body.append(root);

  let st = null;
  let timer = 0;
  let opener = null;
  let gen = 0;

  const atBottom = () => log.scrollHeight - log.scrollTop - log.clientHeight < NEAR;
  const toBottom = () => { log.scrollTop = log.scrollHeight; fresh.hidden = true; };

  function header() {
    const c = h.ctx();
    const d = describeTarget(st.rec, c.accounts, c.sessions);
    // a run opened from its group is named by the run, Back returns to the session
    title.textContent = st.back ? st.runTitle || 'subagent run' : d.title;
    bar.update(st.target.handle);
    // the rule belongs to the session or member, not to a run opened from its log
    if (quick) quick.update(st.back ? null : st.rec);
    meta.replaceChildren();
    const add = (k, v) => meta.append(el('div', '', el('dt', '', k), el('dd', '', v)));
    add('state', stampNode(d.state, d.stateText));
    add('account', chipNode(d.account));
    add('project', el('span', 'projname', d.project));
    const gone = st.rec.leaving || (st.rec.role === 'main' && st.rec.state === 'ended') || st.rec.role === 'review';
    note.hidden = !gone && !st.idle;
    note.textContent = gone ? 'this run has ended, the log stays readable' : st.idle ? 'no run right now, showing the last one' : '';
  }

  // how = 'first' | 'older' | 'newer': decides where the scroll goes
  function draw(how) {
    const p = st.pager;
    const keep = { bottom: atBottom(), top: log.scrollTop, fromEnd: log.scrollHeight - log.scrollTop };
    const focused = document.activeElement && log.contains(document.activeElement) ? document.activeElement.dataset.key : null;
    log.replaceChildren();
    err.hidden = !p.error;
    errText.textContent = p.error;
    retry.hidden = p.loaded;
    if (!p.loaded) {
      if (!p.error) log.append(el('p', 'quiet', 'loading the activity'));
      return;
    }
    if (p.next || p.olderError) {
      const more = el('button', 'older', p.loadingOlder ? 'Loading' : 'Load older');
      more.type = 'button';
      more.disabled = !canLoadOlder(p);
      more.dataset.key = 'older';
      more.addEventListener('click', loadOlder);
      log.append(more);
      if (p.olderError) { const e = el('p', 'err inline', errorText(new Error(p.olderError))); e.setAttribute('role', 'alert'); log.append(e); }
    }
    const rows = buildRows(p.entries, { flat: !!st.target.agent });
    if (!rows.length) log.append(el('p', 'quiet empty', 'Nothing logged for this run yet. New activity shows up here on its own.'));
    else {
      renderRows(log, rows, st.view);
      if (!p.next && !p.olderError) log.append(el('p', 'quiet end', 'start of the log'));
    }
    if (focused) for (const b of log.querySelectorAll('button[data-key]')) if (b.dataset.key === focused) { b.focus({ preventScroll: true }); break; }
    if (how === 'first' || (how === 'newer' && keep.bottom)) toBottom();
    else if (how === 'older') log.scrollTop = log.scrollHeight - keep.fromEnd;
    else log.scrollTop = keep.top;
  }

  const fail = (e) => { st.pager = { ...st.pager, error: errorText(e) }; draw('keep'); };

  async function load() {
    const my = ++gen;
    const t = st.target;
    st.pager = { ...newPager(t) };
    draw('first');
    try {
      const reply = await fetchIt(activityPath({ handle: t.handle, agent: t.agent, limit: PAGE }));
      if (my !== gen || !st || !matches(st.pager, reply)) return;
      st.pager = applyFirst(st.pager, reply);
      draw('first');
    } catch (e) { if (my === gen && st) fail(e); }
    schedule();
  }

  async function refresh() {
    timer = 0;
    if (!st) return;
    retarget();
    if (!st.pager.loaded) { schedule(); return; }
    if (!shouldRefresh({ open: !root.hidden, hidden: document.hidden, loaded: st.pager.loaded, refreshing: st.pager.refreshing })) return;
    const my = gen;
    st.pager = { ...st.pager, refreshing: true };
    try {
      const t = st.target;
      const got = await fetchNewer((params) => fetchIt(activityPath(params)), t, st.pager.newest, { limit: PAGE });
      if (my !== gen || !st) return;
      st.pager = { ...st.pager, refreshing: false };
      if (!matches(st.pager, got)) return;
      // more arrived than the pages allow: start over from the newest page
      if (!got.complete) { load(); return; }
      const r = applyNewer(st.pager, got);
      st.pager = r.pager;
      if (r.added) {
        const wasBottom = atBottom();
        draw('newer');
        if (!wasBottom) fresh.hidden = false;
      } else if (err.hidden === false) draw('keep');
    } catch (e) { if (my === gen && st) { st.pager = { ...st.pager, refreshing: false }; fail(e); } }
    schedule();
  }

  function schedule() {
    clearTimeout(timer);
    timer = 0;
    if (!st || root.hidden || document.hidden) return;
    timer = setTimeout(refresh, EVERY);
  }

  // a team member can move to another run: follow it
  function retarget() {
    header();
    if (st.rec.role !== 'team' || st.back) return;
    const c = h.ctx();
    const t = activityTarget(st.rec, c.sessions);
    st.idle = !t;
    if (t && pagerKey(t) !== pagerKey(st.target)) { st.target = t; st.view = newView(); load(); }
  }

  // a run clicked in the main log opens as its own log, Back returns to the main one
  function openRun(agent, runTitle) {
    if (!st || st.back || !agent) return;
    st.back = st.target;
    st.runTitle = runTitle || '';
    st.target = { handle: st.target.handle, agent, kind: 'run' };
    st.view = newView();
    backBtn.hidden = false;
    header();
    load();
    title.focus();
  }

  function goBack() {
    if (!st || !st.back) return;
    st.target = st.back;
    st.back = null;
    st.runTitle = '';
    st.view = newView();
    backBtn.hidden = true;
    header();
    load();
    title.focus();
  }

  async function loadOlder() {
    if (!st || !canLoadOlder(st.pager)) return;
    const my = gen;
    const t = st.target;
    const before = st.pager.next;
    st.pager = startOlder(st.pager);
    draw('keep');
    try {
      const reply = await fetchIt(activityPath({ handle: t.handle, agent: t.agent, before, limit: PAGE }));
      if (my !== gen || !st || !matches(st.pager, reply)) return;
      st.pager = applyOlder(st.pager, reply);
      draw('older');
    } catch (e) { if (my === gen && st) { st.pager = failOlder(st.pager, e && e.message ? e.message : 'failed'); draw('keep'); } }
  }

  function open(rec) {
    const c = h.ctx();
    const target = activityTarget(rec, c.sessions);
    if (!target) return;
    if (root.hidden) {
      const a = document.activeElement;
      opener = a && a !== document.body && !root.contains(a) ? a : null;
    }
    const same = st && !root.hidden && pagerKey(st.target) === pagerKey(target);
    st = same ? Object.assign(st, { rec }) : { rec, target, back: null, idle: false, view: newView(), pager: newPager(target) };
    backBtn.hidden = !st.back;
    root.hidden = false;
    document.body.classList.add('activity-open');
    header();
    if (!same) load(); else schedule();
    title.focus();
  }

  function close() {
    if (root.hidden) return;
    gen++;
    clearTimeout(timer);
    timer = 0;
    root.hidden = true;
    document.body.classList.remove('activity-open');
    st = null;
    bar.update('');
    const back = opener && opener.isConnected ? opener : h.fallback;
    opener = null;
    if (back) back.focus({ preventScroll: true });
    if (h.onClose) h.onClose();
  }

  closeBtn.addEventListener('click', close);
  backBtn.addEventListener('click', goBack);
  retry.addEventListener('click', () => { if (st) load(); });
  fresh.addEventListener('click', () => { toBottom(); });
  log.addEventListener('scroll', () => { if (atBottom()) fresh.hidden = true; });
  // capture phase so the camera and card handlers never see the Esc that closes the drawer
  addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || root.hidden || document.querySelector('dialog[open]')) return;
    e.preventDefault();
    e.stopPropagation();
    close();
  }, true);
  document.addEventListener('visibilitychange', () => {
    if (!st || root.hidden) return;
    if (document.hidden) { clearTimeout(timer); timer = 0; } else refresh();
  });

  return { open, close, isOpen: () => !root.hidden };
}
