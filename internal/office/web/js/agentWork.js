import * as THREE from 'three';
import { ARM } from './agentConst.js';
import { reduced } from './behave/motion.js';

const AX = new THREE.Vector3(1, 0, 0);
const AY = new THREE.Vector3(0, 1, 0);
const AZ = new THREE.Vector3(0, 0, 1);
const TAU = Math.PI * 2;
const qUp = new THREE.Quaternion();
const smooth = (a, b, x) => { const t = Math.min(1, Math.max(0, (x - a) / (b - a))); return t * t * (3 - 2 * t); };

// seated work poses for the acts of behave/work.js. every value eases in from the plain
// typing pose over 0.4 s; with reduced motion the pose is in place at once and holds still
export function workPose(a, t, act, arms, rot) {
  const calm = reduced();
  const e = calm ? 1 : smooth(0, 0.4, t - a.workAt);
  const mv = calm ? 0 : 1;
  const b = a.bones;
  const lerp = (from, to) => from + (to - from) * e;
  switch (act) {
    case 'read': {
      // lean in, eyes sweeping the screen
      arms(ARM.type, ARM.type);
      rot(b.torso, AX, 0.16 * e);
      rot(b.head, AX, (0.14 + 0.03 * Math.sin(0.8 * t) * mv) * e);
      rot(b.head, AY, Math.sin(TAU * 0.12 * t) * 0.14 * e * mv);
      break;
    }
    case 'type': {
      const amp = 0.11 * mv * e;
      const f = a.tapHz * 2.4;
      arms(ARM.type + amp * Math.sin(TAU * f * t), ARM.type + amp * Math.sin(TAU * f * t + Math.PI));
      rot(b.torso, AX, 0.08 * e);
      rot(b.head, AX, 0.1 * e);
      break;
    }
    case 'wait': {
      // hands in the lap, leaning back
      arms(lerp(ARM.type, ARM.lap), lerp(ARM.type, ARM.lap));
      rot(b.torso, AX, -0.15 * e);
      rot(b.head, AX, -0.1 * e);
      rot(b.head, AY, Math.sin(TAU * 0.1 * t) * 0.15 * e * mv);
      break;
    }
    case 'web': {
      // one hand on the mouse, head moving as the page scrolls
      arms(ARM.type, ARM.type + 0.05 * Math.sin(TAU * 0.7 * t) * mv);
      rot(b.torso, AX, 0.14 * e);
      rot(b.head, AX, 0.12 * e);
      rot(b.head, AY, Math.sin(TAU * 0.2 * t) * 0.18 * e * mv);
      break;
    }
    case 'fail': {
      // hand to the forehead, head bowed
      arms(ARM.type, lerp(ARM.type, ARM.brow));
      rot(b.torso, AX, 0.1 * e);
      rot(b.head, AX, 0.3 * e);
      rot(b.head, AY, Math.sin(TAU * 1.6 * t) * 0.06 * e * mv);
      break;
    }
    case 'pass': {
      // both arms up, a small wave
      arms(ARM.type, ARM.type);
      const up = ARM.cheer + 0.12 * Math.sin(TAU * 1.5 * t) * mv;
      qUp.setFromAxisAngle(AZ, up);
      b['arm-left'].quaternion.slerp(qUp, e);
      qUp.setFromAxisAngle(AZ, -up);
      b['arm-right'].quaternion.slerp(qUp, e);
      rot(b.torso, AX, -0.05 * e);
      rot(b.head, AX, -0.12 * e);
      break;
    }
    default: arms(ARM.type, ARM.type);
  }
}
