// one animation step: remove agents that left, plan breaks, move the camera, update every agent
import * as THREE from 'three';
import { tickCamera } from './stage.js';
import { planBreaks } from './planner.js';
import { applyIdle } from './idle/apply.js';
import { app, stage, ui, world, relayout } from './app.js';
import { lampTick } from './scene/apply.js';
import { syncDim } from './dim.js';
import { ACCOUNT_TINT } from './model.js';

// a finished agent is dropped once it has walked out of the door and shrunk away
function dropGone() {
  for (const [id, a] of [...app.agents]) {
    if (!(a.s.leaving && a.mode === 'away' && a.scale < 0.02)) continue;
    a.dispose();
    app.agents.delete(id);
    if (a.s.role === 'team') {
      app.members.splice(app.members.indexOf(a.s), 1);
    } else if (a.isMain) {
      if (!a.s.subs.length) app.sessions.splice(app.sessions.indexOf(a.s), 1);
    } else {
      const m = a.s.session;
      m.subs.splice(m.subs.indexOf(a.s), 1);
      if (m.leaving && !m.subs.length && !app.agents.has(m.id)) app.sessions.splice(app.sessions.indexOf(m), 1);
    }
    relayout();
  }
}

// leaving runs that never had a character (mapped onto a member, or hidden interns) are dropped without a walk
function dropUnseen() {
  for (const m of [...app.sessions]) {
    for (const x of [...m.subs]) {
      if (!x.leaving || app.agents.has(x.id)) continue;
      m.subs.splice(m.subs.indexOf(x), 1);
      if (m.leaving && !m.subs.length && !app.agents.has(m.id)) app.sessions.splice(app.sessions.indexOf(m), 1);
    }
  }
}

// dotted lines from subagents to their main agent, off unless the toggle is on
function updateLines() {
  const live = new Set();
  for (const s of app.sessions) {
    const main = app.agents.get(s.id);
    for (const sub of s.subs) {
      const a = app.agents.get(sub.id);
      if (!a || !main || sub.leaving || a.mode === 'away' || a.hidden) continue;
      live.add(sub.id);
      let line = app.lines.get(sub.id);
      if (!line) {
        line = new THREE.Line(new THREE.BufferGeometry().setFromPoints([new THREE.Vector3(), new THREE.Vector3()]),
          new THREE.LineDashedMaterial({ color: ACCOUNT_TINT[s.account], dashSize: 0.06, gapSize: 0.05, toneMapped: false }));
        line.frustumCulled = false;
        app.lines.set(sub.id, line);
        world.add(line);
      }
      const p = line.geometry.attributes.position;
      const from = a.flair.position;
      const focus = main.slot.focus;
      const to = focus ? new THREE.Vector3(focus.x, 0.9, focus.z) : main.flair.position;
      p.setXYZ(0, from.x, from.y - 0.05, from.z);
      p.setXYZ(1, to.x, to.y, to.z);
      p.needsUpdate = true;
      line.computeLineDistances();
    }
  }
  for (const [id, line] of app.lines) {
    line.visible = app.linesOn && live.has(id);
    if (!live.has(id)) { world.remove(line); line.geometry.dispose(); line.material.dispose(); app.lines.delete(id); }
  }
}

// ping-pong balls: a rally while both players are at the table, otherwise it rests near the net
function pongBall() {
  for (const p of app.pongs) {
    const A = app.occupant(p.a);
    const B = app.occupant(p.b);
    if (A && B && A.act === 'pong' && B.act === 'pong') {
      const T = 1.6;
      const ph = (app.time % T) / T;
      const dir = ph < 0.5 ? ph * 2 : 2 - ph * 2;
      p.ball.position.x = -0.95 + 1.9 * dir;
      p.ball.position.y = 0.45 + Math.sin(dir * Math.PI) * 0.2 - 0.1;
      p.ball.position.z = 0;
    } else p.ball.position.set(0.15, 0.385, 0.1);
  }
}

export function step(dt) {
  app.time += dt;
  dropGone();
  dropUnseen();
  applyIdle(app);
  planBreaks(app);
  syncDim();
  const near = stage.view.dist < stage.base.dist * 0.7;
  // the nav manager drives the hall camera (the garden ticks it itself), the build editor pauses it while open
  const hall = app.layoutJson && app.layoutJson.v !== 2;
  if (stage.nav && hall) { stage.nav.pause(!!app.editing); stage.nav.update(dt); } else if (!stage.nav) tickCamera(stage, app.dragging);
  app.cutBusy = !!(hall && app.cutaway && app.cutaway.update(stage.camera, dt));
  for (const [id, a] of app.agents) {
    if (a.isMain && a.s.state === 'needs-you' && a.prevState !== 'needs-you' && a.slot.stand) {
      const p = a.slot.stand[a.slot.stand.length - 1];
      app.glance = { x: p.x, z: p.z, start: app.time, dur: 2.4 };
    }
    a.prevState = a.s.state;
    if (a.s.leaving && !a.wasLeaving && a.s.space === 'review') app.printers[0]?.userData.print(app.time);
    a.wasLeaving = a.s.leaving;
    const slot = app.layout.slots.get(id);
    if (!slot) continue;
    a.update(dt, app.time, slot, app.layout, app);
    // main tags always, subagent tags when zoomed in, hovered, or the toggle is on
    a.tag.visible = a.isMain || app.tagsAll || near || a === app.hover;
  }
  for (const f of app.ticks) f(app.time);
  app.life?.tick(dt, app.time);
  pongBall();
  if (app.linesOn || app.lines.size) updateLines();
  ui.setStatus(app.sessions, app.hidden);
  ui.place();
  lampTick(app.time);
}
