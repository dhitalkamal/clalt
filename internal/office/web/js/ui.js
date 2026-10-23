import * as THREE from 'three';
import { el, clear, age } from './dom.js';
import { MAX_SESSIONS } from './model.js';

const $ = (id) => document.getElementById(id);
const tmp = new THREE.Vector3();
const barClass = (pct) => (pct >= 85 ? 'red' : pct >= 60 ? 'amber' : 'green');

// one limit bar. a window the feed has no reading of shows a question mark, which is not 0%.
function bar(name, win) {
  const b = el('div', 'bar', el('span', 'lbl', name));
  if (!win) { b.append(el('em', '', '?')); return b; }
  const pct = Math.max(0, Math.min(100, Math.round(win.usedPct)));
  const fill = el('i', barClass(pct));
  fill.style.width = pct + '%';
  b.append(fill, el('em', '', pct + '%'));
  return b;
}

export function renderWing(div, label, limits) {
  div.append(el('div', 'zname', label), bar('5h', limits && limits.fiveHour), bar('7d', limits && limits.sevenDay));
}

const dl = (rows) => {
  const d = el('dl');
  for (const [k, v] of rows) d.append(el('dt', '', k), el('dd', '', v));
  return d;
};

const stateCell = (state, text) => el('span', '', el('span', 'dot ' + state), text);

export class UI {
  constructor(camera, canvas) {
    this.camera = camera;
    this.canvas = canvas;
    this.labels = new Map();
    this.accounts = new Map();
    this.layer = $('labels');
    this.card = $('card');
    this.status = $('status');
    this.empty = $('empty');
    this.notice = $('notice');
    this.cardKey = '';
  }

  setNotice(text) {
    this.notice.hidden = !text;
    this.notice.textContent = text || '';
  }

  setStatus(sessions, hidden = 0) {
    const live = sessions.filter((s) => !s.leaving);
    const subs = live.flatMap((s) => s.subs.filter((a) => !a.leaving));
    const working = live.filter((s) => s.state === 'busy').length + subs.length;
    const need = live.filter((s) => s.state === 'needs-you').length;
    const sig = `${live.length}/${working}/${need}/${hidden}/${!!this.loaded}`;
    if (sig === this.sig) return;
    this.sig = sig;
    clear(this.status);
    this.status.append(el('b', '', String(live.length)), ' sessions ', el('span', 'sep', '/'), ' ', el('b', '', String(working)),
      ' agents working ', el('span', 'sep', '/'), ' ', el('b', need ? 'warn' : '', String(need)), ' need you');
    if (hidden) this.status.append(el('span', 'sep', '/'), ` ${hidden} more not shown (max ${MAX_SESSIONS})`);
    this.empty.hidden = live.length > 0 || !this.loaded;
  }

  // items: [{id, cls, key, render(div), x, y, z}]. an element is rebuilt only when its key changes.
  setLabels(items) {
    const ids = new Set(items.map((i) => i.id));
    for (const [id, rec] of this.labels) if (!ids.has(id)) { rec.el.remove(); this.labels.delete(id); }
    for (const it of items) {
      let rec = this.labels.get(it.id);
      if (!rec) {
        rec = { el: el('div') };
        this.layer.appendChild(rec.el);
        this.labels.set(it.id, rec);
      }
      if (rec.key !== it.key || rec.cls !== it.cls) {
        rec.el.className = it.cls;
        clear(rec.el);
        it.render(rec.el);
        rec.key = it.key;
        rec.cls = it.cls;
      }
      rec.pos = it;
    }
  }

  place() {
    const w = this.canvas.clientWidth;
    const h = this.canvas.clientHeight;
    for (const { el: node, pos } of this.labels.values()) {
      tmp.set(pos.x, pos.y, pos.z).project(this.camera);
      const sc = Math.min(1, w / 1300);
      node.style.transform = `translate(${(tmp.x * 0.5 + 0.5) * w}px, ${(-tmp.y * 0.5 + 0.5) * h}px) translate(-50%, -100%) scale(${sc})`;
      node.style.opacity = tmp.z < 1 ? 1 : 0;
    }
  }

  // the card is rebuilt only when its content changes, then moved with the pointer
  fill(key, build, x, y) {
    this.card.hidden = false;
    if (key !== this.cardKey) {
      this.cardKey = key;
      clear(this.card);
      build(this.card);
    }
    this.card.style.left = Math.max(8, Math.min(x + 16, innerWidth - this.card.offsetWidth - 8)) + 'px';
    this.card.style.top = Math.max(8, Math.min(y + 16, innerHeight - this.card.offsetHeight - 8)) + 'px';
  }

  showRegion(info, x, y) {
    this.fill('r:' + info.title + info.staff, (c) => c.append(el('div', 'h', info.title), dl([['agents here', String(info.staff)]])), x, y);
  }

  showCard(agent, x, y) {
    if (!agent) { this.card.hidden = true; this.cardKey = ''; return; }
    const s = agent.s;
    const acct = this.accounts.get(s.account);
    const label = acct ? acct.label : s.account;
    if (agent.isMain) {
      const reason = s.state === 'needs-you' && s.reason ? ` (${s.reason.replace(/_/g, ' ')})` : '';
      const subs = s.subs.filter((a) => !a.leaving).length;
      const key = ['m', s.id, s.state, s.reason, s.project, s.model, subs, age(Date.now() - s.startedAt)].join('|');
      this.fill(key, (c) => c.append(el('div', 'h', 'claude (main) ', el('span', 'crown', 'crown')), dl([
        ['handle', s.handle], ['project', s.project], ['model', s.model], ['state', stateCell(s.state, s.state + reason)],
        ['account', label], ['subagents', String(subs)], ['running', age(Date.now() - s.startedAt)]])), x, y);
      return;
    }
    const key = ['s', s.id, s.leaving, s.task, s.space].join('|');
    this.fill(key, (c) => c.append(el('div', 'h', s.type), dl([
      ['task', el('span', 'task', s.task || '-')], ['works for', `${s.session.handle} (${s.session.project})`],
      ['space', s.space], ['state', stateCell('busy', s.leaving ? 'finishing' : 'working')]])), x, y);
  }
}
