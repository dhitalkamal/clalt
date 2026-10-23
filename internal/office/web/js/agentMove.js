import { SEAT_Y, CHAR_SCALE, STAND_YAW, WALK_SPEED, LIFT, angleDiff } from './agentConst.js';
import { mixin } from './mixin.js';

// walking, sitting, standing and leaving: how an agent gets between the door, its seat and the floor
class Move {
  play(name, fade = 0.2) {
    if (this.playing === name) return;
    const next = this.actions[name];
    next.reset().play();
    if (this.playing) next.crossFadeFrom(this.actions[this.playing], fade, false);
    this.playing = name;
  }

  withY(pts) { return pts.map((p) => ({ x: p.x, z: p.z, y: p.y ?? this.slot.y })); }

  follow(kind, fn, done) {
    this.path = { kind, fn, wp: 0, done };
    this.mode = 'walking';
    this.play('walk', kind === 'in' ? 0.01 : 0.2);
  }

  place(p, yaw) {
    this.model.position.set(p.x, p.y ?? this.slot.y, p.z);
    this.model.rotation.set(0, yaw, 0);
    this.model.scale.setScalar(CHAR_SCALE);
  }

  resetBones() { for (const n of ['head', 'torso']) this.bones[n].quaternion.copy(this.base[n]); }

  detach() { this.model.parent?.remove(this.model); this.world.add(this.model); }

  hideProps() { this.cup.visible = false; this.paper.visible = false; this.book.visible = false; this.paddle.visible = false; this.cupHeld = false; }

  leaveChair() {
    const c = this.slot.chair;
    this.detach();
    this.place({ x: c.x, z: c.z, y: this.slot.y }, this.slot.rotY);
    this.resetBones();
  }

  startWalkIn() {
    const r = this.slot.node.route;
    const first = this.L.path(this.L.spawn, this.slot.node);
    this.detach();
    this.place(first[0], Math.atan2(first[1].x - first[0].x, first[1].z - first[0].z));
    this.follow('in', () => this.L.path(this.L.spawn, this.slot.node), () => this.sit());
    void r;
  }

  startOut() {
    let from;
    if (this.mode === 'seated') { this.leaveChair(); from = () => this.L.path(this.slot.node, this.L.spawn); }
    else if (this.mode === 'standing') {
      const last = this.withY([this.slot.stand[this.slot.stand.length - 1]]);
      from = () => [...last, ...this.L.path(this.slot.corr, this.L.spawn)];
    } else if (this.mode === 'leisure') {
      const node = this.curNode || this.slot.node;
      this.hideProps();
      this.model.position.y = this.slot.y === 0 ? 0 : this.model.position.y;
      from = () => this.L.path(node, this.L.spawn);
    } else from = () => this.L.path(this.slot.node, this.L.spawn);
    this.finishOut(from);
  }

  finishOut(from) {
    this.plan = null;
    this.follow('out', from, () => {
      this.model.parent?.remove(this.model);
      this.mode = 'away';
      this.playing = null;
      this.hideProps();
      this.mixer.stopAllAction();
    });
  }

  startStand() {
    this.leaveChair();
    this.follow('stand', () => this.withY([this.slot.chair, ...this.slot.stand]), () => {
      this.mode = 'standing';
      this.model.rotation.y = this.slot.rotY + STAND_YAW;
      this.play('idle');
    });
  }

  instantStand() {
    const last = this.withY([this.slot.stand[this.slot.stand.length - 1]])[0];
    this.detach();
    this.place(last, this.slot.rotY + STAND_YAW);
    this.mode = 'standing';
    this.play('idle', 0.01);
  }

  startBack() {
    this.follow('back', () => this.withY([this.slot.chair, ...this.slot.stand]).reverse(), () => this.sit());
  }

  sit() {
    this.mode = 'seated';
    this.seat.add(this.model);
    this.model.position.set(0, SEAT_Y, -0.03);
    this.model.rotation.set(0, 0, 0);
    this.model.scale.setScalar(CHAR_SCALE);
    this.play('sit', 0.2);
    this.curNode = this.slot.node;
    if (this.cupHeld) this.cup.visible = true;
  }

  walkTo(p, dt) {
    const m = this.model.position;
    const py = p.y ?? m.y;
    const dx = p.x - m.x;
    const dz = p.z - m.z;
    const dist = Math.hypot(dx, dz);
    if (dist < 1e-4 && Math.abs(py - m.y) > 1e-3) {
      const dy = py - m.y;
      const step = LIFT * dt;
      if (Math.abs(dy) <= step) m.y = py; else m.y += Math.sign(dy) * step;
      return Math.abs(py - m.y) < 1e-4;
    }
    const step = WALK_SPEED * dt;
    if (dist <= step) { m.x = p.x; m.z = p.z; m.y = py; return true; }
    m.x += (dx / dist) * step;
    m.z += (dz / dist) * step;
    // stairs: height follows horizontal progress so it arrives with the waypoint
    m.y += (py - m.y) * (step / dist);
    this.model.rotation.y += angleDiff(Math.atan2(dx, dz), this.model.rotation.y) * Math.min(1, dt * 10);
    return false;
  }

  stepPath(dt) {
    const p = this.path;
    const pts = p.fn();
    const target = pts[p.wp + 1];
    if (!target) { this.finishPath(); return; }
    if (this.walkTo(target, dt)) {
      p.wp++;
      if (p.wp >= pts.length - 1) this.finishPath();
    }
  }

  finishPath() {
    const done = this.path.done;
    this.path = null;
    done();
  }
}

export function installMove(Agent) { mixin(Agent, Move); }
