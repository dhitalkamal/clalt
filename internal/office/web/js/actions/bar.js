// the action bar of the activity drawer: Move to, Open terminal, Stop (with a confirm step).
// pure decisions live in availability.js, plan.js and text.js; this file only draws and sends.
import { el } from '../dom.js';
import { actionView } from './availability.js';
import { accountLabel } from '../tool.js';
import { rankAccounts } from './plan.js';
import { confirmStop, sendAction } from './text.js';

// h: { ctx() -> {accounts, sessions}, send: optional (verb, body) -> {ok, reason} }
export function createActionBar(h) {
  const send = h.send || sendAction;
  const root = el('div', 'actions');
  root.hidden = true;
  const state = { mode: 'idle', msg: '', bad: false, busy: false };
  let handle = '';
  let lastKey = '';

  const button = (label, key, onClick, off) => {
    const b = el('button', '', label);
    b.type = 'button';
    b.dataset.key = key;
    if (off && !off.enabled) { b.disabled = true; b.title = off.why; } else b.addEventListener('click', onClick);
    return b;
  };

  async function run(verb, body, okText) {
    state.busy = true;
    state.mode = 'idle';
    state.msg = '';
    draw(true);
    const r = await send(verb, body);
    state.busy = false;
    state.bad = !r.ok;
    state.msg = r.ok ? okText : r.reason;
    draw(true);
  }

  function draw(force) {
    const c = h.ctx();
    const s = c.sessions.find((x) => x.handle === handle && !x.leaving);
    const view = s ? actionView(s.actions) : actionView({ move: false, focus: false, stop: false, why: 'this session has ended' });
    const label = (id) => accountLabel(c.accounts, id);
    const choices = s ? rankAccounts(c.accounts, s.account) : [];
    const key = JSON.stringify([handle, view, state, choices, s && s.project, s && s.account]);
    if (!force && key === lastKey) return;
    lastKey = key;
    const focused = root.contains(document.activeElement) ? document.activeElement.dataset.key : null;
    root.replaceChildren();
    const row = el('div', 'arow');
    if (state.mode === 'confirm') {
      const text = el('p', 'confirm', confirmStop(s.project, label(s.account)));
      text.setAttribute('role', 'alert');
      row.append(text, button('Stop', 'stop-yes', () => run('stop', { handle }, 'Stopping: the session ends after the current step.')),
        button('Cancel', 'stop-no', () => { state.mode = 'idle'; draw(true); }));
    } else {
      row.append(
        button('Move to', 'move', () => { state.mode = state.mode === 'pick' ? 'idle' : 'pick'; draw(true); }, state.busy ? { enabled: false, why: 'working' } : view.move),
        button('Open terminal', 'focus', () => run('focus', { handle }, 'Brought the terminal to the front.'), state.busy ? { enabled: false, why: 'working' } : view.focus),
        button('Stop', 'stop', () => { state.mode = 'confirm'; draw(true); }, state.busy ? { enabled: false, why: 'working' } : view.stop));
    }
    root.append(row);
    if (state.mode === 'pick') {
      const list = el('div', 'picker');
      list.setAttribute('role', 'group');
      list.setAttribute('aria-label', 'move to account');
      if (!choices.length) list.append(el('p', 'quiet', 'there is no other account'));
      choices.forEach((a, i) => {
        const pct = a.pct === null ? '5h ?' : '5h ' + a.pct + '%';
        const b = button(a.label + ' ' + pct + (i === 0 && !a.soon && a.pct !== null ? ' (most room)' : ''), 'to:' + a.id,
          () => run('move', { handle, account: a.id }, 'Asked to move to ' + a.label + ' at its next pause.'));
        if (a.soon) b.title = 'this account is about to hit its limit too';
        list.append(b);
      });
      root.append(list);
    }
    const why = [view.move, view.focus, view.stop].find((v) => !v.enabled);
    if (why && state.mode !== 'confirm') root.append(el('p', 'quiet why', why.why));
    if (state.msg) {
      const m = el('p', state.bad ? 'msg bad' : 'msg', state.msg);
      m.setAttribute('role', state.bad ? 'alert' : 'status');
      root.append(m);
    }
    if (focused) for (const b of root.querySelectorAll('button[data-key]')) if (b.dataset.key === focused) { b.focus({ preventScroll: true }); break; }
  }

  return {
    el: root,
    // called when the drawer shows another session, or on each header refresh
    update(next) {
      if (next !== handle) { handle = next; Object.assign(state, { mode: 'idle', msg: '', bad: false, busy: false }); lastKey = ''; }
      root.hidden = !handle;
      if (handle) draw(false);
    },
  };
}
