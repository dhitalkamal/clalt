// draws rows from rows.js into the log. text only: every string goes in through textContent,
// nothing from a transcript is ever parsed as markup. toggles change their own node in place,
// so a focused button stays focused.
import { el } from '../dom.js';

// view: {expanded: Set of row ids, opened: Set of group ids, openRun: optional fn(agent id)}
export function renderRows(parent, rows, view, nested = false) {
  for (const r of rows) parent.append(rowNode(r, view, nested));
}

const stamp = (r) => {
  const t = el('time', 'when', r.time);
  if (r.iso) { t.dateTime = r.iso; t.title = r.iso; }
  return t;
};

function button(cls, label, key) {
  const b = el('button', cls, label);
  b.type = 'button';
  b.dataset.key = key;
  return b;
}

function rowNode(r, view, nested) {
  if (r.type === 'text') return textNode(r, view);
  if (r.type === 'tool') return toolNode(r);
  if (r.type === 'result') return resultNode(r, view);
  return groupNode(r, view, nested);
}

function textNode(r, view) {
  const body = el('div', 'body');
  const node = el('article', 'row text ' + r.role, el('div', 'who', el('span', '', r.label), stamp(r)), body);
  const show = () => { body.textContent = view.expanded.has(r.id) ? r.full : r.text; };
  show();
  if (r.cut) {
    const more = button('more', '', r.id + ':more');
    const label = () => { more.textContent = view.expanded.has(r.id) ? 'Show less' : 'Show more (' + r.hidden + ' more characters)'; more.setAttribute('aria-expanded', String(view.expanded.has(r.id))); };
    label();
    more.addEventListener('click', () => { view.expanded.has(r.id) ? view.expanded.delete(r.id) : view.expanded.add(r.id); show(); label(); });
    node.append(more);
  }
  return node;
}

const statusNode = (s) => el('span', 'st ' + s, s === 'failed' ? 'failed' : 'ok');

function toolNode(r) {
  const node = el('div', 'row tool ' + r.status, el('span', 'tn', r.tool));
  if (r.files.text) { const f = el('span', 'files', r.files.text); f.title = r.files.title; node.append(f); }
  if (r.detail) node.append(el('span', 'detail', r.detail));
  node.append(el('span', 'meta', r.duration), statusNode(r.status), stamp(r));
  return node;
}

function resultNode(r, view) {
  const text = el('span', 'detail', r.snippet || '(empty)');
  const node = el('div', 'row result ' + r.status, el('span', 'tn', 'result'), text, statusNode(r.status), stamp(r));
  if (!r.cut) return node;
  const full = el('pre', 'full');
  full.hidden = true;
  const more = button('more', 'Show full output', r.id + ':more');
  more.setAttribute('aria-expanded', 'false');
  const sync = () => {
    const on = view.expanded.has(r.id);
    full.hidden = !on;
    if (on && !full.textContent) full.textContent = r.full;
    more.textContent = on ? 'Hide output' : 'Show full output';
    more.setAttribute('aria-expanded', String(on));
  };
  sync();
  more.addEventListener('click', () => { view.expanded.has(r.id) ? view.expanded.delete(r.id) : view.expanded.add(r.id); sync(); });
  node.append(more, full);
  return node;
}

let serial = 0;
function groupNode(r, view) {
  const bodyId = 'act-g-' + ++serial;
  const meta = [r.state === 'running' ? 'running' : r.status === 'failed' ? 'failed' : 'done', r.count + (r.count === 1 ? ' step' : ' steps'), r.duration].filter(Boolean).join(', ');
  const head = button('ghead', '', r.id);
  head.setAttribute('aria-controls', bodyId);
  const caret = el('span', 'caret');
  caret.setAttribute('aria-hidden', 'true');
  head.append(caret, el('span', 'gt', r.title), el('span', 'meta', meta), stamp(r));
  const body = el('div', 'gbody');
  body.id = bodyId;
  const sync = () => {
    const on = view.opened.has(r.id);
    head.setAttribute('aria-expanded', String(on));
    caret.textContent = on ? '-' : '+';
    body.hidden = !on;
    if (on && !body.childNodes.length) {
      if (r.children.length) renderRows(body, r.children, view, true);
      else if (view.openRun) {
        // the main log holds only the start and end of a run, its steps are in its own log
        const go = button('more', 'Open this run\'s log', r.id + ':run');
        go.addEventListener('click', () => view.openRun(r.agent));
        body.append(el('p', 'quiet', 'the steps of this run are in its own log'), go);
      } else body.append(el('p', 'quiet', 'no steps logged for this run'));
    }
  };
  sync();
  head.addEventListener('click', () => { view.opened.has(r.id) ? view.opened.delete(r.id) : view.opened.add(r.id); sync(); });
  return el('section', 'row group ' + r.state, head, body);
}
