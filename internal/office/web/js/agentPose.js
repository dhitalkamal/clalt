import * as THREE from 'three';
import { ARM } from './agentConst.js';
import { workPose } from './agentWork.js';

const AX = new THREE.Vector3(1, 0, 0);
const AY = new THREE.Vector3(0, 1, 0);
const AZ = new THREE.Vector3(0, 0, 1);
const TAU = Math.PI * 2;
const qT = new THREE.Quaternion();
const qA = new THREE.Quaternion();
const qB = new THREE.Quaternion();
const qC = new THREE.Quaternion();

const smooth = (a, b, x) => { const t = Math.min(1, Math.max(0, (x - a) / (b - a))); return t * t * (3 - 2 * t); };
const angleDiff = (a, b) => Math.atan2(Math.sin(a - b), Math.cos(a - b));
// raised-and-lowered envelope over 0..1
export const bump = (w) => smooth(0, 0.18, w) * (1 - smooth(0.82, 1, w));

// busy pauses: a short deterministic schedule per agent, period and offset fixed at spawn
function pauseState(a, t) {
  const period = a.pausePeriod;
  const u = (t + a.pauseOff) % period;
  const len = 3.0;
  if (u < period - len) return null;
  const w = (u - (period - len)) / len;
  return { kind: Math.floor((t + a.pauseOff) / period) % 3, e: bump(w) };
}

export function applyPose(a, t) {
  const b = a.bones;
  const rot = (bone, axis, ang) => bone.quaternion.multiply(qT.setFromAxisAngle(axis, ang));
  for (const n of ['head', 'torso']) b[n].quaternion.copy(a.base[n]);
  const arms = (dL, dR) => {
    b['arm-left'].quaternion.copy(qA.setFromAxisAngle(AX, dL)).multiply(qB.setFromAxisAngle(AY, -(Math.PI / 2)));
    b['arm-right'].quaternion.copy(qA.setFromAxisAngle(AX, dR)).multiply(qB.setFromAxisAngle(AY, Math.PI / 2));
  };
  const tap = (off) => Math.sin(TAU * a.tapHz * t + off);
  a.pausing = null;

  if (a.mode === 'standing') {
    // needs-you: right arm raised and swinging, whole body bouncing
    b['arm-right'].quaternion.setFromAxisAngle(AZ, -1.35 + Math.sin(TAU * 1.5 * t) * 0.4);
    a.model.position.y = a.slot.y + Math.abs(Math.sin(TAU * 1.4 * t)) * 0.03;
    rot(b.torso, AZ, Math.sin(TAU * 1.4 * t) * 0.05);
    return;
  }
  if (a.mode === 'leisure') { leisure(a, t, arms, rot); return; }

  const st = a.s.state;
  // a subagent just handed in its work: short thankful nod
  if (a.nodUntil && t < a.nodUntil) rot(b.head, AX, 0.14 * bump(1 - (a.nodUntil - t) / 1.8));
  if (st === 'idle') {
    const cup = a.cupHeld;
    rot(b.torso, AX, -0.2);
    rot(b.head, AY, Math.sin(TAU * 0.08 * t) * 0.3);
    if (cup) {
      const w = ((t + a.pauseOff) % 7) / 7;
      const e = bump(Math.min(1, w * 3.2)) * (w < 0.31 ? 1 : 0);
      arms(ARM.lap, ARM.lap + (ARM.mouth - ARM.lap) * e);
      rot(b.head, AX, -0.12 * e);
    } else arms(ARM.lap, ARM.lap);
    glance(a, t, rot);
    return;
  }
  // busy. a current piece of work (read, edit, wait, ...) replaces the plain pose and the random pauses
  if (a.workAct) {
    workPose(a, t, a.workAct, arms, rot);
    glance(a, t, rot);
    return;
  }
  const p = pauseState(a, t);
  const e = p ? p.e : 0;
  a.pausing = p && e > 0.05 ? p.kind : null;
  const amp = 0.05 * (1 - e);
  let dR = ARM.type + amp * tap(Math.PI);
  let dL = ARM.type + amp * tap(0);
  rot(b.head, AX, 0.03 + 0.03 * Math.sin(TAU * 0.25 * t));
  rot(b.head, AY, 0.3 * (a.side || 1) * (1 - 0.6 * e));
  if (p) {
    if (p.kind === 0) { dR += (ARM.chin - dR) * e; rot(b.head, AX, -0.1 * e); rot(b.head, AY, 0.25 * e); }
    else if (p.kind === 1) { dL += (ARM.lap - dL) * e; dR += (ARM.lap - dR) * e; rot(b.torso, AX, -0.28 * e); rot(b.head, AX, -0.12 * e); }
    else { dR += (ARM.mouth - dR) * e; rot(b.head, AX, -0.12 * e); }
  }
  arms(dL, dR);
  glance(a, t, rot);
}

// head turn toward a recent event (a main agent calling for attention)
function glance(a, t, rot) {
  const g = a.env?.glance;
  if (!g || a.s.state === 'needs-you') return;
  const w = (t - g.start) / g.dur;
  if (w < 0 || w > 1) return;
  const rigYaw = a.rig.rotation.y + (a.model.parent === a.seat ? a.seat.rotation.y : 0);
  const want = Math.atan2(g.x - a.model.getWorldPosition(vTmp).x, g.z - vTmp.z);
  const d = Math.max(-1.0, Math.min(1.0, angleDiff(want, rigYaw)));
  rot(a.bones.head, AY, d * bump(w));
}
const vTmp = new THREE.Vector3();

// bits that depend on the act of a break step. every motion is a smooth function of time since the act began.
function leisure(a, t, arms, rot) {
  const b = a.bones;
  const u = t - a.actStart;
  const w = Math.min(1, u / Math.max(0.01, a.actEnd - a.actStart));
  const sipE = (period, delay = 1.0) => { const x = ((u - delay) % period + period) % period / period; return u > delay ? bump(Math.min(1, x * 3.6)) * (x < 0.28 ? 1 : 0) : 0; };
  rot(b.head, AY, Math.sin(TAU * 0.1 * t + a.ph) * 0.2);
  switch (a.act) {
    case 'stretch': {
      const e = bump(w);
      b['arm-left'].quaternion.setFromAxisAngle(AZ, ARM.stretch * e);
      b['arm-right'].quaternion.setFromAxisAngle(AZ, -ARM.stretch * e);
      rot(b.torso, AX, -0.18 * e);
      rot(b.head, AX, -0.2 * e);
      break;
    }
    case 'order': {
      // reach toward the machine, then hold the cup near the chest
      const reach = smooth(0, 0.25, w) * (1 - smooth(0.55, 0.75, w));
      arms(ARM.hold, ARM.hold + (ARM.type - ARM.hold) * reach);
      break;
    }
    case 'sip': case 'carry': {
      const e = sipE(5.0, 1.2);
      arms(ARM.hold, ARM.hold + (ARM.mouth - ARM.hold) * e);
      rot(b.head, AX, -0.14 * e);
      if (a.chatting) {
        const nod = Math.sin(TAU * 0.5 * t + a.ph) * 0.07 * smooth(0.5, 1, Math.sin(TAU * 0.2 * t + a.ph) * 0.5 + 0.5);
        rot(b.head, AX, nod);
        a.bones.head.quaternion.multiply(qC.setFromAxisAngle(AY, 0));
      }
      break;
    }
    case 'cooler': {
      const fill = smooth(0, 0.2, w) * (1 - smooth(0.4, 0.55, w));
      const drink = bump(Math.min(1, Math.max(0, (w - 0.55) / 0.4)));
      arms(ARM.hold, ARM.hold + (ARM.type - ARM.hold) * fill + (ARM.mouth - ARM.hold) * drink);
      rot(b.head, AX, -0.14 * drink);
      break;
    }
    case 'window': {
      arms(ARM.lap, ARM.lap);
      rot(b.head, AY, 0.15 * Math.sin(TAU * 0.15 * t));
      break;
    }
    case 'arcade': {
      arms(ARM.game + 0.05 * Math.sin(TAU * 1.7 * t + a.ph), ARM.game + 0.05 * Math.sin(TAU * 2.1 * t));
      rot(b.torso, AX, 0.06);
      rot(b.head, AX, 0.05);
      break;
    }
    case 'read': {
      arms(ARM.hold - 0.1, ARM.hold - 0.1);
      rot(b.head, AX, 0.2);
      rot(b.head, AY, Math.sin(TAU * 0.05 * t) * 0.1);
      break;
    }
    case 'pong': {
      // swing when the ball reaches this player, the rally phase comes from the shared clock
      const sw = a.rallySwing(t);
      arms(ARM.hold, ARM.swing - 0.5 * sw);
      rot(b.torso, AY, 0.18 * sw * (a.spot.name === 'pongA' ? 1 : -1));
      break;
    }
    case 'handoff': {
      arms(ARM.hold, ARM.hold + (ARM.type - ARM.hold) * bump(Math.min(1, w * 2)));
      rot(b.head, AX, 0.08 * bump(w));
      break;
    }
    default: arms(ARM.lap, ARM.lap);
  }
}
