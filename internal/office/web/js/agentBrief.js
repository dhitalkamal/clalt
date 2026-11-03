import { mixin } from './mixin.js';
import { replanPath } from './garden/carry.js';

const BRIEF_SECS = 2;
const BRIEF_KEEP = 20;

// a main agent walks to a new subagent's desk, briefs it and walks back
class Brief {
  queueBrief(sub) {
    (this.briefQueue ||= []).push({ sub, until: this.now + BRIEF_KEEP });
  }

  // the next queued subagent that still has a desk to walk to. one still walking in stays queued,
  // so the main does not brief an empty chair
  pickBrief() {
    const q = this.briefQueue || [];
    while (q.length) {
      const e = q[0];
      const t = this.env.agents.get(e.sub);
      if (e.until < this.now || !t || t.s.leaving || !t.slot || !t.slot.corr) { q.shift(); continue; }
      if (t.mode !== 'seated') return null;
      q.shift();
      return t;
    }
    return null;
  }

  startBrief() {
    const t = this.pickBrief();
    if (!t) return;
    this.leaveChair();
    const from = this.slot.node;
    const id = t.s.id;
    const goal = (a) => (a.env.agents.get(id) || t).slot;
    this.follow('brief', () => this.L.path(from, goal(this).corr), () => {
      const at = goal(this);
      const m = this.model.position;
      this.curNode = at.corr;
      this.model.rotation.y = Math.atan2(at.chair.x - m.x, at.chair.z - m.z);
      this.beginAct('brief', BRIEF_SECS, null);
    }, (L, a) => goal(a).corr);
  }

  tickBrief(t) {
    if (t >= this.actEnd || this.s.state === 'needs-you') this.endBrief();
  }

  endBrief() {
    const node = this.curNode;
    this.follow('back', () => this.L.path(node, this.slot.node), () => this.sit(), (L, a) => a.slot.node);
    this.act = 'walk';
  }

  // needs-you or leaving cuts the walk to the desk short: back to the chair from where it stands
  abandonBrief(gone) {
    if (!this.path || this.path.kind !== 'brief' || !(gone || this.s.state === 'needs-you')) return;
    const m = this.model.position;
    const pts = replanPath(this.L, { x: m.x, y: m.y, z: m.z }, this.slot.node);
    this.follow('back', () => pts, () => this.sit(), (L, a) => a.slot.node);
  }
}

export function installBrief(Agent) { mixin(Agent, Brief); }
