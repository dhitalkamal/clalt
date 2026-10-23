// break plans, handoffs and the small moments, as methods mixed into Agent.
// everything is driven by timers (actStart / actEnd) and eased envelopes, nothing is random per frame.
const SIT_Y = 0.12;

export function installBreaks(Agent) {
  const P = Agent.prototype;

  // props follow the act and state
  P.syncProps = function syncProps() {
    const leisure = this.mode === 'leisure';
    const w = leisure ? (this.now - this.actStart) / Math.max(0.01, this.actEnd - this.actStart) : 0;
    if (leisure && this.act === 'cooler' && w > 0.3) this.cupHeld = true;
    if (leisure && this.act === 'order' && w > 0.5) { this.cupHeld = true; if (!this.cupHotUntil || this.cupHotUntil < this.now) { this.cupHotUntil = this.now + 25; this.env.hotRef.until = this.now + 25; } }
    if (this.cupHeld && this.cupHideAt && this.now > this.cupHideAt && this.mode === 'seated') { this.cupHeld = false; this.cupHideAt = 0; }
    this.cup.visible = !!this.cupHeld && this.mode !== 'away';
    this.paper.visible = !!this.carryPaper || (leisure && this.act === 'handoff');
    this.book.visible = leisure && this.act === 'read';
    this.paddle.visible = leisure && this.act === 'pong';
    // chatting: both sides of a table pair are sipping at the same time
    this.chatting = false;
    this.chatSpeaker = false;
    if (leisure && this.spot && this.spot.partner) {
      const other = this.env.occupant(this.spot.partner);
      if (other && other.act === this.act) {
        this.chatting = this.act === 'sip';
        const turn = Math.floor(this.now / 3.2 + (this.spot.name < this.spot.partner ? 0 : 1)) % 2;
        this.chatSpeaker = this.chatting && turn === 0 && (this.now - this.actStart) > 2;
      }
    }
  };

  P.rallySwing = function rallySwing(t) {
    const other = this.spot && this.env.occupant(this.spot.partner);
    if (!other || other.act !== 'pong') return 0;
    const T = 1.6;
    const ph = ((t + (this.spot.name === 'pongA' ? 0 : T / 2)) % T) / T;
    if (ph > 0.18) return 0;
    const x = ph / 0.18;
    return Math.sin(Math.PI * x);
  };

  P.beginAct = function beginAct(act, dur, spot) {
    this.mode = 'leisure';
    this.act = act;
    this.actStart = this.now;
    this.actEnd = this.now + dur;
    this.spot = spot || null;
    this.curSpot = spot ? spot.name : this.curSpot;
    if (spot) {
      this.model.position.set(spot.x, spot.sit ? SIT_Y : 0, spot.z);
      this.model.rotation.set(0, spot.yaw, 0);
      this.play(spot.sit ? 'sit' : 'idle', 0.3);
    } else this.play('idle', 0.3);
  };

  P.startBreak = function startBreak() {
    this.plan = this.planOffer;
    this.planOffer = null;
    this.stepIdx = -1;
    this.abort = false;
    this.leaveChair();
    this.curNode = this.slot.node;
    this.curSpot = null;
    this.beginAct('stretch', 2.4, null);
  };

  // jump straight into step i of a plan (used for the seeded scene)
  P.instantBreak = function instantBreak(plan, i) {
    this.plan = plan;
    this.planOffer = null;
    this.stepIdx = i;
    this.abort = false;
    this.detach();
    const st = plan.steps[i];
    const spot = this.L.spots[st.spot];
    if (plan.steps.slice(0, i).some((s) => s.act === 'order')) { this.cupHeld = true; this.cupHotUntil = this.now + 25; }
    this.curNode = spot.node;
    this.beginAct(st.act, st.dur, spot);
    this.actStart = this.now - st.dur * 0.35;
  };

  P.tickLeisure = function tickLeisure(t) {
    this.syncProps();
    if (!this.plan && this.act !== 'handoff') return;
    const abort = this.abort || (this.s.state !== 'idle' && this.act !== 'handoff');
    if (this.act === 'handoff') {
      if (t >= this.actEnd) {
        const node = this.curNode;
        this.carryPaper = false;
        this.finishOut(() => this.L.path(node, this.L.spawn));
      }
      return;
    }
    if (!abort && t < this.actEnd) return;
    this.nextStep(abort);
  };

  P.nextStep = function nextStep(abort) {
    const plan = this.plan;
    this.stepIdx++;
    const st = plan.steps[this.stepIdx];
    const from = this.curNode;
    this.curSpot = null;
    this.spot = null;
    this.model.position.y = this.slot.y;
    if (!abort && st) {
      const spot = this.L.spots[st.spot];
      this.follow('break', () => this.L.path(from, spot.node), () => { this.curNode = spot.node; this.beginAct(st.act, st.dur, spot); });
      return;
    }
    const carry = plan.carry && !abort;
    this.follow('back', () => this.L.path(from, this.slot.node), () => {
      this.plan = null;
      this.sit();
      this.breakCount = (this.breakCount || 0) + 1;
      this.nextBreakAt = this.now + 18 + (this.s.seed % 7);
      if (carry && this.cupHeld) this.cupHideAt = this.now + 16;
      else { this.cupHeld = false; }
    });
    this.act = 'walk';
    if (!carry && !(this.cupHeld && plan.carry)) { /* cup is dropped at the desk, see sit() callback */ }
  };

  // a finished subagent walks to its main agent's cabin door, hands over a paper, then leaves through the lobby
  P.startHandoff = function startHandoff() {
    const main = this.env.agents.get(this.s.session.id);
    if (!main || main.s.leaving || main.s.state === 'ended' || !main.slot.corr) { this.startOut(); return; }
    this.leaveChair();
    this.carryPaper = true;
    const from = this.slot.node;
    const ms = main.slot;
    this.follow('handoff', () => this.L.path(from, ms.corr), () => {
      this.curNode = ms.corr;
      this.model.rotation.y = Math.PI;
      this.beginAct('handoff', 2.6, null);
      main.nodUntil = this.now + 1.8;
      this.env.glance = null;
    });
  };
}
