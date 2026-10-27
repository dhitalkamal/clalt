import * as THREE from 'three';
import { applyPose } from './agentPose.js';
import { installBreaks } from './breaks.js';
import { installBuild } from './agentBuild.js';
import { disposeTree } from './dispose.js';
import { installMove } from './agentMove.js';
import { ACCOUNT_TINT } from './model.js';
import { recColor, isTeamRec, isInternRec, makeInternLook, tickMemberLook, disposeLook } from './garden/teamlook.js';
import { TAU, BUBBLE_Y, angleDiff } from './agentConst.js';
import { makeBlob, placeBlob, stripCast } from './agentShadow.js';

export class Agent {
  constructor(s, world, slot, look) {
    this.s = s;
    this.world = world;
    this.slot = slot;
    this.L = null;
    this.env = null;
    this.isMain = s.role === 'main';
    this.tint = (s.tint != null || s.color) ? recColor(s) : ACCOUNT_TINT[s.account] || 0x7a8aa8;
    this.mode = 'away';
    this.path = null;
    this.now = 0;
    // per-character constants, chosen once at spawn and never changed
    this.ph = ((s.seed * 0.37) % 1) * TAU;
    this.tapHz = 2 + ((s.seed * 0.13) % 1);
    this.pausePeriod = 16 + (s.seed % 5) * 2;
    this.pauseOff = (s.seed * 5.3) % this.pausePeriod;
    this.scale = 0.001;
    this.plan = null;
    this.nextBreakAt = 5 + (s.seed % 4) * 3;
    this.rig = new THREE.Group();
    this.rig.position.set(slot.x, slot.y, slot.z);
    this.rig.rotation.y = slot.rotY;
    this.rig.scale.setScalar(0.001);
    this.buildRig();
    this.buildChar(look);
    this.buildFlair();
    world.add(this.rig, this.flair);
    if (isInternRec(s)) makeInternLook(this, recColor(s));
    this.blob = makeBlob(world);
    if (this.blob) stripCast(this);
  }

  update(dt, t, slot, L, env) {
    if (!slot) return; // hidden interns have no seat
    this.slot = slot;
    this.L = L;
    this.env = env;
    this.now = t;
    const k = 1 - Math.exp(-dt * 6);
    this.rig.position.x += (slot.x - this.rig.position.x) * k;
    this.rig.position.z += (slot.z - this.rig.position.z) * k;
    this.rig.position.y = slot.y;
    this.rig.rotation.y += angleDiff(slot.rotY, this.rig.rotation.y) * k;
    const gone = this.s.leaving || this.s.state === 'ended';
    const target = this.s.leaving && this.mode === 'away' ? 0.001 : 1;
    this.scale += (target - this.scale) * k;
    this.rig.scale.setScalar(Math.max(this.scale, 0.001));

    if (!gone && this.mode === 'away') this.startWalkIn();
    if (gone && this.mode === 'seated') { if (!this.isMain && this.s.leaving) this.startHandoff(); else this.startOut(); }
    else if (gone && (this.mode === 'standing' || (this.mode === 'leisure' && this.act !== 'handoff'))) this.startOut();
    if (!gone && this.mode === 'seated') {
      if (this.s.state === 'needs-you' && slot.stand) this.startStand();
      else if (this.s.state === 'idle' && this.planOffer && t >= this.nextBreakAt) this.startBreak();
    }
    if (this.mode === 'standing' && this.s.state !== 'needs-you' && slot.kind !== 'stand') this.startBack();
    if (this.mode === 'leisure') this.tickLeisure(t);
    if (this.mode === 'walking') this.stepPath(dt);

    this.mixer.update(dt);
    const c = t + this.ph;
    if (this.mode === 'seated' || this.mode === 'standing' || this.mode === 'leisure') applyPose(this, t);
    this.detail.userData.tick?.(t);
    this.updateScreen(c, gone);
    if (isTeamRec(this.s)) tickMemberLook(this, dt);
    this.updateFlair(t, gone);
    placeBlob(this);
  }

  updateScreen(t, gone) {
    const seated = this.mode === 'seated';
    const busy = seated && this.s.state === 'busy' && !gone && this.pausing == null;
    const shown = this.mode === 'away' || gone ? 'ended' : this.s.state;
    this.screenTex.draw(shown, t);
    const lit = shown === 'ended' ? 0 : shown === 'busy' ? 0.14 : shown === 'needs-you' ? 0.1 : 0.04;
    this.screenGlow.material.opacity = lit;
    this.deskGlow.material.opacity = lit * 0.5;
    this.keys.forEach((m, i) => {
      m.material.opacity = busy ? Math.pow(Math.max(0, Math.sin(TAU * this.tapHz * 0.5 * t + i * 1.9 + this.ph)), 6) * 0.85 : 0;
    });
  }

  updateFlair(t, gone) {
    const f = this.flair;
    const on = this.mode !== 'away' && this.scale > 0.6;
    f.visible = on;
    if (!on) return;
    const head = this.bones.head;
    head.updateWorldMatrix(true, false);
    head.getWorldPosition(f.position);
    const seated = this.mode === 'seated';
    const state = this.s.state;
    this.ring.visible = state === 'busy' && !gone && (seated || this.mode === 'walking') && this.pausing == null;
    this.ring.rotation.y = t * 2.2;
    const pulse = 0.8 + 0.25 * Math.sin(t * 4);
    this.ring.children.forEach((d, i) => d.scale.setScalar(pulse * (0.7 + 0.3 * Math.sin(t * 4 + i))));
    const bob = Math.sin(t * 2.4) * 0.03;
    this.bubble.visible = this.mode === 'standing' && state === 'needs-you';
    this.bubble.position.y = BUBBLE_Y + bob;
    this.bubble.material.opacity = 0.85 + 0.15 * Math.sin(t * 3);
    this.done.visible = this.mode === 'leisure' && this.act === 'handoff' && this.now - this.actStart > 0.9;
    this.done.position.y = BUBBLE_Y + bob;
    this.chat.visible = !!this.chatSpeaker;
    this.chat.position.y = BUBBLE_Y + bob;
    this.cupSteam.visible = this.cup.visible && t < (this.cupHotUntil || 0);
    this.cupSteam.userData.tick?.(t);
    if (this.crown) this.crown.rotation.y = 0.5;
  }

  setHighlight(on) { this.screenMat.color.setScalar?.(on ? 1.25 : 1); }

  dispose() {
    this.world.remove(this.rig, this.model, this.flair);
    if (this.blob) this.world.remove(this.blob);
    this.mixer.stopAllAction();
    this.mixer.uncacheRoot(this.model);
    for (const part of [this.rig, this.model, this.flair]) disposeTree(part);
    this.screenTex.tex.dispose();
    disposeLook(this);
  }
}

installBuild(Agent);
installMove(Agent);
installBreaks(Agent);
