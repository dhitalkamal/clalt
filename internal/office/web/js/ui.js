import * as THREE from 'three';
import { el, clear, age } from './dom.js';
import { MAX_SESSIONS } from './model.js';
import { fitLabels } from './labelfit.js';
import { teamCard } from './cardinfo.js';

const $ = (id) => document.getElementById(id);
const tmp = new THREE.Vector3();
const SVGNS = 'http://www.w3.org/2000/svg';
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
  // zpct is what the compact one-line card shows in place of the bars: the busiest window
  const used = [limits && limits.fiveHour, limits && limits.sevenDay].filter(Boolean).map((w) => Math.round(w.usedPct));
  const worst = used.length ? Math.max(...used) : null;
  const zname = el('div', 'zname', label);
  zname.append(el('span', 'zpct ' + (worst === null ? '' : barClass(worst)), worst === null ? '?' : worst + '%'));
  div.append(zname, bar('5h', limits && limits.fiveHour), bar('7d', limits && limits.sevenDay));
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
    // leader lines live in one svg under the labels: attributes only, no inline style (strict csp)
    this.leaders = document.createElementNS(SVGNS, 'svg');
    this.leaders.setAttribute('class', 'leaders');
    this.layer.prepend(this.leaders);
    this.lines = new Map();
    this.fitPrev = null;
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
        rec.size = null;
      }
      rec.pos = it;
    }
  }

  // full and compact sizes of a label in css px, measured once per content
  sizes(rec) {
    if (rec.size) return rec.size;
    const node = rec.el;
    node.classList.remove('compact');
    const size = { w: node.offsetWidth, h: node.offsetHeight };
    if (rec.cls === 'zone') {
      node.classList.add('compact');
      size.cw = node.offsetWidth;
      size.ch = node.offsetHeight;
      node.classList.remove('compact');
    }
    rec.size = size;
    return size;
  }

  place() {
    const w = this.canvas.clientWidth;
    const h = this.canvas.clientHeight;
    const sc = Math.min(1, w / 1300);
    const items = [];
    for (const [id, rec] of this.labels) {
      tmp.set(rec.pos.x, rec.pos.y, rec.pos.z).project(this.camera);
      rec.sx = (tmp.x * 0.5 + 0.5) * w;
      rec.sy = (-tmp.y * 0.5 + 0.5) * h;
      rec.front = tmp.z < 1;
      if (!rec.front) continue;
      const z = this.sizes(rec);
      items.push({ id, x: rec.sx, y: rec.sy, w: z.w * sc, h: z.h * sc, cw: z.cw ? z.cw * sc : 0, ch: z.ch ? z.ch * sc : 0, pri: rec.cls === 'zone' ? 1 : 0 });
    }
    const fit = fitLabels(items, { prev: this.fitPrev });
    this.fitPrev = fit;
    const live = new Set();
    for (const [id, rec] of this.labels) {
      const f = (rec.front && fit.get(id)) || { dx: 0, dy: 0, compact: false, hidden: false };
      rec.el.classList.toggle('compact', f.compact);
      rec.el.style.transform = `translate(${rec.sx + f.dx}px, ${rec.sy + f.dy}px) translate(-50%, -100%) scale(${sc})`;
      rec.el.style.opacity = rec.front && !f.hidden ? 1 : 0;
      if (f.leader) { this.leader(id, rec, f, items.find((i) => i.id === id)); live.add(id); }
    }
    for (const [id, line] of this.lines) if (!live.has(id)) { line.remove(); this.lines.delete(id); }
  }

  // a short line from the anchor to the nearest point of its displaced label
  leader(id, rec, f, it) {
    let line = this.lines.get(id);
    if (!line) {
      line = document.createElementNS(SVGNS, 'line');
      line.setAttribute('class', 'leader');
      this.leaders.appendChild(line);
      this.lines.set(id, line);
    }
    const w = f.compact ? it.cw : it.w;
    const h = f.compact ? it.ch : it.h;
    const cx = rec.sx + f.dx;
    const cy = rec.sy + f.dy;
    const ex = Math.max(cx - w / 2, Math.min(rec.sx, cx + w / 2));
    const ey = Math.max(cy - h, Math.min(rec.sy, cy));
    // an anchor still under its card needs no line
    line.setAttribute('visibility', Math.hypot(ex - rec.sx, ey - rec.sy) < 4 ? 'hidden' : 'visible');
    line.setAttribute('x1', rec.sx.toFixed(1));
    line.setAttribute('y1', rec.sy.toFixed(1));
    line.setAttribute('x2', ex.toFixed(1));
    line.setAttribute('y2', ey.toFixed(1));
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
    this.fill('r:' + info.title + JSON.stringify(info.rows), (c) => c.append(el('div', 'h', info.title), dl(info.rows)), x, y);
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
    if (s.role === 'team' || s.intern) {
      const info = teamCard(s);
      const rows = info.rows.map(([k, v]) => [k, typeof v === 'string' ? v : stateCell(v.state, v.text)]);
      this.fill(info.key, (c) => c.append(el('div', 'h', info.title), dl(rows)), x, y);
      return;
    }
    const key = ['s', s.id, s.leaving, s.task, s.space].join('|');
    this.fill(key, (c) => c.append(el('div', 'h', s.type), dl([
      ['task', el('span', 'task', s.task || '-')], ['works for', `${s.session.handle} (${s.session.project})`],
      ['space', s.space], ['state', stateCell('busy', s.leaving ? 'finishing' : 'working')]])), x, y);
  }
}
