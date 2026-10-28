// the reviews inbox: a left drawer with the list of reviews, and one review (findings by severity, the
// summary, "Open run log"). the run log opens in the activity panel through openRun(rec).
import { el, clear } from '../dom.js';
import { fetchReviews, fetchReview } from './api.js';
import { chipNode, toneFlag } from '../notebook.js';
import { inboxRows, groupFindings, runLogRec, lastCheckedText } from './model.js';

const EVERY = 10000;

export function createInbox(h) {
  const root = el('aside', 'reviews');
  root.setAttribute('role', 'dialog');
  root.setAttribute('aria-modal', 'false');
  root.setAttribute('aria-labelledby', 'reviews-title');
  root.hidden = true;
  const back = el('button', 'back', 'Back');
  back.type = 'button';
  back.hidden = true;
  const title = el('h2', '', 'PR reviews');
  title.id = 'reviews-title';
  title.tabIndex = -1;
  const close = el('button', 'close', 'Close');
  close.type = 'button';
  close.setAttribute('aria-label', 'Close PR reviews');
  const status = el('p', 'quiet');
  const err = el('div', 'err');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const body = el('div', 'rbody');
  body.tabIndex = 0;
  root.append(el('header', '', back, title, close), status, err, body);
  document.body.append(root);

  let opener = null;
  let timer = 0;
  let detail = null;
  let last = null;

  const setErr = (m) => { err.textContent = m || ''; err.hidden = !m; };

  function drawList() {
    clear(body);
    title.textContent = 'PR reviews';
    back.hidden = true;
    if (!last) return;
    status.textContent = (last.running ? 'reviewing ' + last.running.repo + '#' + last.running.number + ' now. ' : '') + lastCheckedText(last.lastChecked, Date.now());
    const rows = inboxRows(last, Date.now());
    if (!rows.length) body.append(el('p', 'quiet', last.enabled ? 'no reviews yet. pull requests that ask for your review show up here.' : 'PR reviews are off. Turn them on in Settings.'));
    for (const r of rows) {
      const flag = toneFlag(r.tone);
      const tail = el('span', 'tail', flag ? el('span', flag.cls, flag.text) : null, el('span', 'sum', r.summary));
      const b = el('button', 'item ' + r.tone, el('span', 'itl', r.title), el('span', 'meta', r.ref + ' by ' + r.author + ', ' + r.when + ' ago', r.account ? chipNode(r.account) : null), tail);
      b.type = 'button';
      b.title = r.tip;
      b.addEventListener('click', () => openOne(r.id));
      body.append(b);
    }
  }

  function drawDetail(r) {
    clear(body);
    title.textContent = r.title;
    back.hidden = false;
    status.textContent = '';
    const meta = el('dl', 'info');
    const add = (k, v) => meta.append(el('div', '', el('dt', '', k), el('dd', '', v)));
    add('pull request', r.repo + '#' + r.number);
    add('author', r.author || '-');
    add('commit', (r.sha || '').slice(0, 7));
    if (r.account) add('account', chipNode(r.account));
    body.append(meta);
    if (r.error) body.append(el('p', 'err inline', 'the review failed: ' + r.error));
    if (r.skipped) body.append(el('p', 'quiet', r.skipped));
    if (r.summary) body.append(el('p', 'summary', r.summary));
    for (const g of groupFindings(r.findings)) {
      const sec = el('section', 'fgroup ' + g.severity, el('h3', '', g.severity + ' (' + g.items.length + ')'));
      for (const f of g.items) {
        sec.append(el('div', 'finding', f.where ? el('code', 'where', f.where) : null, el('strong', '', f.title), f.detail ? el('p', '', f.detail) : null));
      }
      body.append(sec);
    }
    if (!r.error && !r.skipped && !r.findings.length) body.append(el('p', 'quiet', 'no findings'));
    const rec = runLogRec(r);
    if (rec) {
      const run = el('button', 'primary runlog', 'Open run log');
      run.type = 'button';
      run.addEventListener('click', () => h.openRun(rec));
      body.append(run);
    }
  }

  async function refresh() {
    try {
      last = await fetchReviews();
      setErr('');
      h.onList(last);
      if (!detail) drawList();
    } catch (e) {
      setErr('could not load the reviews: ' + e.message);
    }
  }

  async function openOne(id) {
    try {
      detail = await fetchReview(id);
      setErr('');
      drawDetail(detail);
      title.focus();
    } catch (e) {
      setErr('could not load this review: ' + e.message);
    }
  }

  function tick() {
    clearTimeout(timer);
    if (root.hidden) return;
    timer = setTimeout(async () => { await refresh(); tick(); }, EVERY);
  }

  function open(from) {
    if (root.hidden) opener = from || null;
    root.hidden = false;
    detail = null;
    refresh().then(() => { title.focus(); });
    tick();
  }
  function shut() {
    root.hidden = true;
    detail = null;
    clearTimeout(timer);
    if (opener && opener.isConnected) opener.focus();
  }
  back.addEventListener('click', () => { detail = null; drawList(); title.focus(); });
  close.addEventListener('click', shut);
  root.addEventListener('keydown', (e) => { if (e.key === 'Escape') { e.stopPropagation(); detail ? back.click() : shut(); } });

  return { open, close: shut, isOpen: () => !root.hidden, refresh };
}
