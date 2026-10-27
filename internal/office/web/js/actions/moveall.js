// Move all: confirm, then one move request per session. a small dialog over the page.
import { el } from '../dom.js';
import { planMoveAll } from './plan.js';
import { confirmMoveAll, sendAction } from './text.js';

let box = null;

// ctx() -> {accounts, sessions}; fromId: the account running out
export function openMoveAll(ctx, fromId, send = sendAction) {
  const c = ctx();
  const from = (c.accounts.get(fromId) || {}).label || fromId;
  const plan = planMoveAll(c.sessions, fromId, c.accounts);
  close();
  const root = el('div', 'moveall');
  root.setAttribute('role', 'alertdialog');
  root.setAttribute('aria-label', 'move all sessions');
  const text = el('p', '', '');
  const msg = el('p', 'quiet', '');
  msg.setAttribute('role', 'status');
  const go = el('button', '', 'Move all');
  const no = el('button', '', 'Cancel');
  go.type = no.type = 'button';
  no.addEventListener('click', close);
  if (!plan.handles.length || !plan.target) {
    text.textContent = !plan.handles.length ? 'No session of ' + from + ' can be moved from here.' : 'There is no other account with room to move to.';
    go.hidden = true;
    no.textContent = 'Close';
  } else {
    text.textContent = confirmMoveAll(plan.handles.length, from, plan.target);
    if (plan.skipped) msg.textContent = plan.skipped + (plan.skipped === 1 ? ' session stays' : ' sessions stay') + ' (they cannot be moved from here).';
    go.addEventListener('click', async () => {
      go.disabled = true;
      let ok = 0;
      const bad = [];
      for (const handle of plan.handles) {
        const r = await send('move', { handle, account: plan.target.id });
        if (r.ok) ok++; else bad.push(r.reason);
      }
      text.textContent = 'Asked ' + ok + ' of ' + plan.handles.length + ' to move to ' + plan.target.label + ' at ' + (plan.handles.length === 1 ? 'its' : 'their') + ' next pause.';
      msg.textContent = bad.length ? 'Refused: ' + [...new Set(bad)].join('; ') : '';
      go.hidden = true;
      no.textContent = 'Close';
      no.focus();
    });
  }
  root.append(text, msg, el('div', 'arow', go, no));
  document.body.append(root);
  box = root;
  (go.hidden ? no : go).focus();
}

export function close() {
  if (box) box.remove();
  box = null;
}
