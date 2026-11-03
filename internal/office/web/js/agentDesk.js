import { mixin } from './mixin.js';
import { reduced } from './behave/motion.js';
import { briefReady } from './behave/brief.js';
import { canStand } from './behave/ask.js';

// what a seated or standing agent decides each frame: stand to ask for help, break, brief, sit back.
// with reduced motion nobody walks for it: the agent stands or sits in place and briefs are skipped
class Desk {
  tickDesk(t, gone) {
    const calm = reduced();
    if (calm) this.briefQueue = null;
    if (this.mode === 'walking') this.abandonBrief(gone);
    const st = this.s.state;
    if (!gone && this.mode === 'seated') {
      if (st === 'needs-you' && canStand(this.slot)) { if (calm) this.instantStand(); else this.startStand(); }
      else if (st === 'idle' && this.planOffer && t >= this.nextBreakAt) this.startBreak();
      else if (briefReady(this, calm)) this.startBrief();
    }
    if (this.mode === 'standing' && st !== 'needs-you' && this.slot.kind !== 'stand') { if (calm) this.sit(); else this.startBack(); }
  }
}

export function installDesk(Agent) { mixin(Agent, Desk); }
