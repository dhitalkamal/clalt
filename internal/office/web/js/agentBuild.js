import * as THREE from 'three';
import { furniture, character } from './assets.js';
import { ScreenTex, bubbleTexture } from './screen.js';
import { makeStool, makeCrown, makeRing, textSprite, glowSprite, makeMug } from './props.js';
import { deskDetails, monitorExtras, steamPuffs, box } from './details.js';
import { SEAT_OFF } from './layout.js';
import { mixin } from './mixin.js';
import { CHAR_SCALE, K, TAG_Y, BUBBLE_Y } from './agentConst.js';

let bubbleTex = null;
let doneTex = null;
let dotsTex = null;

function iconTex(draw) {
  const cv = document.createElement('canvas');
  cv.width = cv.height = 128;
  const c = cv.getContext('2d');
  draw(c);
  const t = new THREE.CanvasTexture(cv);
  t.colorSpace = THREE.SRGBColorSpace;
  t.userData.shared = true;
  return t;
}

const bubbleSprite = (map, size) => {
  const s = new THREE.Sprite(new THREE.SpriteMaterial({ map, transparent: true, depthWrite: false, toneMapped: false }));
  s.scale.setScalar(size);
  s.renderOrder = 11;
  s.visible = false;
  return s;
};

// the parts of an agent: desk rig, monitor, keyboard, character and the floating tag and bubbles
class Build {
  buildRig() {
    const { rig, slot } = this;
    const dh = 0.38;
    this.side = slot.side || 1;
    this.seat = new THREE.Group();
    this.seat.position.set(0, 0, -SEAT_OFF);
    const stool = makeStool(this.tint, this.s.seed % 3);
    this.seat.add(stool);
    rig.add(this.seat);
    // an agent without a seat stands: no stool, desk or monitor
    if (slot.kind === 'stand') this.seat.remove(stool);
    this.holder = new THREE.Group();
    this.holder.position.z = slot.kind === 'round' ? 0.3 : 0;
    rig.add(this.holder);
    if (slot.kind === 'desk') {
      const desk = furniture('desk');
      desk.rotation.y = Math.PI;
      rig.add(desk);
      // side return that carries the monitor, angled toward the typist and the camera
      const ret = new THREE.Group();
      const top = box(0.4, 0.03, 0.6, 0xd9b48a);
      top.position.y = dh - 0.015;
      ret.add(top);
      for (const [lx, lz] of [[-0.17, -0.25], [0.17, -0.25], [-0.17, 0.25], [0.17, 0.25]]) { const l = box(0.03, dh - 0.03, 0.03, 0x8a949c); l.position.set(lx, (dh - 0.03) / 2, lz); ret.add(l); }
      ret.position.set(this.side * 0.565, 0, -0.2);
      rig.add(ret);
    }
    this.buildMonitor(dh, slot.kind === 'round');
    this.buildKeyboard(dh);
    this.detail = deskDetails(this.s.seed, dh, slot.kind === 'round');
    this.holder.add(this.detail);
    if (slot.kind === 'stand') this.holder.visible = false;
    rig.traverse((o) => { o.userData.agent = this; });
  }

  // monitor on the side return, screen turned about 55 degrees: toward the typist and still readable from the front camera
  buildMonitor(dh, round) {
    const mon = new THREE.Group();
    const sx = round ? 0.3 * this.side : this.side * 0.565;
    mon.position.set(sx, dh, round ? 0.0 : -0.25);
    mon.rotation.y = -this.side * 0.96;
    mon.scale.setScalar(1.05);
    mon.add(furniture('computerScreen'));
    mon.add(monitorExtras(this.s.seed));
    this.screenTex = new ScreenTex(this.s.seed);
    this.screenMat = new THREE.MeshBasicMaterial({ map: this.screenTex.tex, toneMapped: false });
    this.screenPlane = new THREE.Mesh(new THREE.PlaneGeometry(0.31, 0.19), this.screenMat);
    this.screenPlane.position.set(0, 0.16, 0.052);
    mon.add(this.screenPlane);
    this.screenGlow = glowSprite(0xffc98a, 0.9, 0);
    this.screenGlow.position.set(0, 0.17, 0.2);
    mon.add(this.screenGlow);
    this.deskGlow = new THREE.Mesh(new THREE.PlaneGeometry(0.5, 0.35),
      new THREE.MeshBasicMaterial({ map: this.screenGlow.material.map, color: 0xffb870, transparent: true, opacity: 0,
        depthWrite: false, blending: THREE.AdditiveBlending, toneMapped: false }));
    this.deskGlow.rotation.x = -Math.PI / 2;
    this.deskGlow.position.set(0, 0.004, 0.25);
    mon.add(this.deskGlow);
    this.holder.add(mon);
  }

  buildKeyboard(dh) {
    const kb = new THREE.Group();
    kb.position.set(0, dh, -0.06);
    kb.rotation.y = Math.PI;
    kb.add(furniture('computerKeyboard'));
    this.keys = [];
    for (let i = 0; i < 5; i++) {
      const m = new THREE.Mesh(new THREE.PlaneGeometry(0.035, 0.03), new THREE.MeshBasicMaterial({
        color: 0xffe2a8, transparent: true, opacity: 0, depthWrite: false, blending: THREE.AdditiveBlending, toneMapped: false }));
      m.rotation.x = -Math.PI / 2;
      m.position.set(-0.09 + i * 0.045, 0.034, ((i * 7) % 3 - 1) * 0.03);
      kb.add(m);
      this.keys.push(m);
    }
    this.holder.add(kb);
    const mouse = furniture('computerMouse');
    mouse.position.set(-0.22, dh, -0.06);
    this.holder.add(mouse);
  }

  buildChar(look) {
    const { model, clips } = character(look);
    this.model = model;
    model.scale.setScalar(CHAR_SCALE);
    this.mixer = new THREE.AnimationMixer(model);
    this.actions = {};
    for (const n of ['idle', 'walk', 'sit']) this.actions[n] = this.mixer.clipAction(clips.find((c) => c.name === n));
    this.bones = {};
    for (const n of ['arm-left', 'arm-right', 'head', 'torso']) this.bones[n] = model.getObjectByName(n);
    this.base = {};
    for (const n of ['head', 'torso']) this.base[n] = this.bones[n].quaternion.clone();
    // hand props ride on the right arm bone so they follow every arm pose
    const hand = this.bones['arm-right'];
    this.cup = makeMug(0xf6f1e8);
    this.cup.scale.setScalar(1.6);
    this.cup.position.set(-0.11, -0.02, 0.03);
    this.cup.visible = false;
    this.cupSteam = steamPuffs(0xffffff, 0.1);
    this.cupSteam.position.set(0, 0.04, 0);
    this.cup.add(this.cupSteam);
    hand.add(this.cup);
    this.paper = new THREE.Mesh(new THREE.BoxGeometry(0.07, 0.09, 0.008), new THREE.MeshStandardMaterial({ color: 0xf2eee4 }));
    this.paper.position.set(-0.12, -0.02, 0.04);
    this.paper.visible = false;
    hand.add(this.paper);
    this.book = new THREE.Mesh(new THREE.BoxGeometry(0.08, 0.1, 0.02), new THREE.MeshStandardMaterial({ color: 0x4f86c6 }));
    this.book.position.set(-0.11, -0.02, 0.05);
    this.book.visible = false;
    hand.add(this.book);
    this.paddle = new THREE.Group();
    const blade = new THREE.Mesh(new THREE.CylinderGeometry(0.04, 0.04, 0.008, 14), new THREE.MeshStandardMaterial({ color: 0xd9534f }));
    blade.rotation.x = Math.PI / 2;
    blade.position.set(0, 0.0, 0.0);
    const grip = new THREE.Mesh(new THREE.BoxGeometry(0.012, 0.05, 0.012), new THREE.MeshStandardMaterial({ color: 0xc89b6a }));
    grip.position.y = -0.045;
    this.paddle.add(blade, grip);
    this.paddle.position.set(-0.13, 0.0, 0.05);
    this.paddle.visible = false;
    hand.add(this.paddle);
    this.playing = null;
    model.userData.agent = this;
    model.traverse((o) => { o.userData.agent = this; });
  }

  buildFlair() {
    const f = new THREE.Group();
    this.flair = f;
    if (this.isMain) { this.crown = makeCrown(); this.crown.position.y = 0.27 * K; f.add(this.crown); }
    this.ring = makeRing(this.tint);
    this.ring.position.y = 0.42 * K;
    f.add(this.ring);
    this.tag = this.isMain ? textSprite('claude', this.tint, this.s.project) : textSprite(this.s.type, this.tint);
    this.tag.position.y = TAG_Y;
    f.add(this.tag);
    if (!bubbleTex) { bubbleTex = bubbleTexture(); bubbleTex.userData.shared = true; }
    this.bubble = bubbleSprite(bubbleTex, 0.34);
    if (!doneTex) {
      doneTex = iconTex((c) => {
        c.fillStyle = 'rgba(255,255,255,0.96)'; c.beginPath(); c.arc(64, 64, 52, 0, 7); c.fill();
        c.strokeStyle = '#3aa76d'; c.lineWidth = 12; c.lineCap = 'round'; c.lineJoin = 'round';
        c.beginPath(); c.moveTo(38, 66); c.lineTo(58, 86); c.lineTo(92, 44); c.stroke();
      });
      dotsTex = iconTex((c) => {
        c.fillStyle = 'rgba(255,255,255,0.96)'; c.beginPath(); c.roundRect(14, 30, 100, 56, 22); c.fill();
        c.fillStyle = '#555'; for (const x of [40, 64, 88]) { c.beginPath(); c.arc(x, 58, 7, 0, 7); c.fill(); }
      });
    }
    this.done = bubbleSprite(doneTex, 0.3);
    this.chat = bubbleSprite(dotsTex, 0.3);
    f.add(this.bubble, this.done, this.chat);
    f.visible = false;
  }
}

export function installBuild(Agent) { mixin(Agent, Build); }
