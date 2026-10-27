// the summary panel: a Day tab (picker, sections, copy as text) and a History tab
// (charts per account). state logic and numbers live in the pure modules next to it.
import { el } from '../dom.js';
import { fetchSummary as defaultSummary, fetchHistory as defaultHistory, errorText } from './api.js';
import { localDay, shiftDay, validDay, dayLabel } from './format.js';
import { summaryModel } from './model.js';
import { summaryText } from './copytext.js';
import { renderDay } from './render.js';
import { renderHistory } from './historyview.js';

const RANGES = [7, 30, 90];

function btn(cls, label, onClick) {
  const b = el('button', cls, label);
  b.type = 'button';
  if (onClick) b.addEventListener('click', onClick);
  return b;
}

// copy by the clipboard api, else by a hidden textarea. false when neither worked.
async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.cssText = 'position:fixed;left:-9999px;top:0';
    document.body.append(ta);
    ta.select();
    let ok = false;
    try { ok = document.execCommand('copy'); } catch { ok = false; }
    ta.remove();
    return ok;
  }
}

// h: { fallback: element to focus on close, now(): Date, fetchSummary, fetchHistory, copy }
export function createSummaryPanel(h) {
  const getDay = h.fetchSummary || defaultSummary;
  const getHist = h.fetchHistory || defaultHistory;
  const copy = h.copy || copyText;
  const now = h.now || (() => new Date());
  const st = { tab: 'day', day: localDay(now()), days: 30, text: '', hist: null };
  let gen = 0;
  let opener = null;

  const root = el('aside', 'summary');
  root.setAttribute('role', 'dialog');
  root.setAttribute('aria-modal', 'false');
  root.setAttribute('aria-labelledby', 'summary-title');
  root.hidden = true;
  const title = el('h2', '', 'Daily summary');
  title.id = 'summary-title';
  title.tabIndex = -1;
  const closeBtn = btn('close', 'Close', () => close());
  closeBtn.setAttribute('aria-label', 'Close summary panel');

  const tabs = el('div', 'tabs');
  tabs.setAttribute('role', 'tablist');
  const tabBtns = {};
  for (const [id, label] of [['day', 'Day'], ['history', 'History']]) {
    const b = btn('tab', label, () => setTab(id));
    b.setAttribute('role', 'tab');
    b.id = 'summary-tab-' + id;
    b.setAttribute('aria-controls', 'summary-body');
    tabBtns[id] = b;
    tabs.append(b);
  }
  tabs.addEventListener('keydown', (e) => {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
    e.preventDefault();
    const next = st.tab === 'day' ? 'history' : 'day';
    setTab(next);
    tabBtns[next].focus();
  });

  const toolrow = el('div', 'toolrow');
  const body = el('div', 'body');
  body.id = 'summary-body';
  body.setAttribute('role', 'tabpanel');
  body.tabIndex = 0;
  const status = el('p', 'status');
  status.setAttribute('role', 'status');
  root.append(el('header', '', title, closeBtn), tabs, toolrow, status, body);
  document.body.append(root);

  const say = (t) => { status.textContent = t; };
  const today = () => localDay(now());

  function drawBar() {
    toolrow.replaceChildren();
    if (st.tab === 'day') {
      const prev = btn('nav', 'Previous', () => go(shiftDay(st.day, -1)));
      prev.setAttribute('aria-label', 'Previous day');
      const input = el('input', 'pick');
      input.type = 'date';
      input.value = st.day;
      input.max = today();
      input.setAttribute('aria-label', 'Day');
      input.addEventListener('change', () => { if (validDay(input.value)) go(input.value); });
      const next = btn('nav', 'Next', () => go(shiftDay(st.day, 1)));
      next.setAttribute('aria-label', 'Next day');
      next.disabled = st.day >= today();
      const copyBtn = btn('copy', 'Copy as text', onCopy);
      copyBtn.disabled = !st.text;
      toolrow.append(prev, input, next, copyBtn, el('span', 'daylabel', dayLabel(st.day, today())));
    } else {
      for (const n of RANGES) {
        const b = btn('range', n + ' days', () => { st.days = n; loadHistory(); });
        b.setAttribute('aria-pressed', String(n === st.days));
        toolrow.append(b);
      }
    }
  }

  async function onCopy() {
    if (!st.text) return;
    say((await copy(st.text)) ? 'Copied the summary as text.' : 'Could not copy, select the text yourself.');
  }

  function go(day) {
    if (!day || !validDay(day) || day > today()) return;
    st.day = day;
    loadDay();
  }

  async function loadDay() {
    const my = ++gen;
    st.text = '';
    drawBar();
    body.replaceChildren(el('p', 'quiet', 'Reading the transcripts of ' + st.day));
    say('');
    try {
      const reply = await getDay(st.day);
      if (my !== gen || st.tab !== 'day') return;
      st.text = summaryText(reply);
      drawBar();
      body.replaceChildren(renderDay(summaryModel(reply)));
    } catch (e) {
      if (my === gen) { body.replaceChildren(); say(errorText(e)); }
    }
  }

  function chartWidth() { return Math.max(260, Math.min(640, body.clientWidth - 12)); }

  function drawHistory() {
    body.replaceChildren(renderHistory(st.hist, chartWidth()));
  }

  async function loadHistory() {
    const my = ++gen;
    drawBar();
    body.replaceChildren(el('p', 'quiet', 'Reading the transcripts of the last ' + st.days + ' days'));
    say('');
    try {
      const reply = await getHist(st.days);
      if (my !== gen || st.tab !== 'history') return;
      st.hist = reply;
      drawHistory();
    } catch (e) {
      if (my === gen) { body.replaceChildren(); say(errorText(e)); }
    }
  }

  function setTab(id) {
    st.tab = id;
    for (const [k, b] of Object.entries(tabBtns)) {
      b.setAttribute('aria-selected', String(k === id));
      b.tabIndex = k === id ? 0 : -1;
    }
    body.setAttribute('aria-labelledby', 'summary-tab-' + id);
    if (id === 'day') loadDay(); else loadHistory();
  }

  function open() {
    if (root.hidden) {
      const a = document.activeElement;
      opener = a && a !== document.body && !root.contains(a) ? a : null;
      st.day = today();
      root.hidden = false;
      document.body.classList.add('summary-open');
      setTab('day');
    }
    title.focus();
  }

  function close() {
    if (root.hidden) return;
    gen++;
    root.hidden = true;
    document.body.classList.remove('summary-open');
    const back = opener && opener.isConnected ? opener : h.fallback;
    opener = null;
    if (back) back.focus({ preventScroll: true });
  }

  // capture phase so the camera and card handlers never see the Esc that closes the panel
  addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || root.hidden || document.querySelector('dialog[open]')) return;
    e.preventDefault();
    e.stopPropagation();
    close();
  }, true);
  let resize = 0;
  addEventListener('resize', () => {
    clearTimeout(resize);
    resize = setTimeout(() => { if (!root.hidden && st.tab === 'history' && st.hist) drawHistory(); }, 150);
  });

  return { open, close, toggle: () => (root.hidden ? open() : close()), isOpen: () => !root.hidden };
}
