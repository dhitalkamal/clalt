// the per-frame pass of the idle rules: who is hidden, and who resting in a room goes back to the desk.
// the planner decides who is offered what, this only reacts to the current rules and states.
import { idleChoice, roomsOf } from './resolve.js';

export function applyIdle(app) {
  const ids = roomsOf(app.layoutJson).map((r) => r.id);
  for (const a of app.agents.values()) {
    const choice = idleChoice(a.s, app.idleRules, ids);
    const idle = a.s.state === 'idle';
    a.setHidden(idle && choice === 'hide');
    if (a.plan && a.plan.rest && !(idle && choice.room === a.plan.rest)) a.abort = true;
  }
}
