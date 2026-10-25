// the walker's body for third person: one Kenney character with a tint and a soft ring under it. this is the only
// nav file that needs three.js, ground.js loads it lazily. the character comes from assets.js, so the assets have
// to be loaded already (the page loads them before the office shows).
import * as THREE from 'three';
import { character } from '../assets.js';
import { CHAR_SCALE } from '../agentConst.js';
import { clipFor } from './clipmath.js';

const TINT = 0xffc857;

function ringTexture() {
  const cv = document.createElement('canvas');
  cv.width = cv.height = 64;
  const c = cv.getContext('2d');
  const g = c.createRadialGradient(32, 32, 10, 32, 32, 31);
  g.addColorStop(0, 'rgba(255,200,87,0)');
  g.addColorStop(0.7, 'rgba(255,200,87,0.55)');
  g.addColorStop(1, 'rgba(255,200,87,0)');
  c.fillStyle = g;
  c.fillRect(0, 0, 64, 64);
  return new THREE.CanvasTexture(cv);
}

export function createAvatar(ctx) {
  const root = new THREE.Group();
  const { model, clips } = character('male-c');
  model.scale.setScalar(CHAR_SCALE);
  const own = [];
  // the model shares materials with the loaded asset, so each mesh gets a tinted copy
  model.traverse((o) => {
    if (!o.isMesh) return;
    const m = [].concat(o.material)[0].clone();
    m.color.multiply(new THREE.Color(TINT));
    own.push(m);
    o.material = m;
  });
  const mixer = new THREE.AnimationMixer(model);
  const acts = {};
  for (const n of ['idle', 'walk', 'sprint', 'jump', 'fall']) {
    const c = clips.find((k) => k.name === n);
    if (c) acts[n] = mixer.clipAction(c);
  }
  if (acts.jump) { acts.jump.setLoop(THREE.LoopOnce, 1); acts.jump.clampWhenFinished = true; }
  let playing = null;
  const play = (n) => {
    if (!acts[n] || playing === n) return;
    acts[n].reset().play();
    if (playing) acts[n].crossFadeFrom(acts[playing], 0.15, false);
    playing = n;
  };
  const ringTex = ringTexture();
  const ringMat = new THREE.MeshBasicMaterial({ map: ringTex, transparent: true, depthWrite: false, toneMapped: false });
  const ringGeo = new THREE.PlaneGeometry(1.1, 1.1);
  const ring = new THREE.Mesh(ringGeo, ringMat);
  ring.rotation.x = -Math.PI / 2;
  ring.position.y = 0.02;
  ring.renderOrder = 5;
  root.add(model, ring);
  ctx.stage.scene.add(root);
  play('idle');

  return {
    root,
    // st: {x, y, z, yaw, moving, running, speed, air, vy}. speed is the real ground speed, it sets the clip rate
    update(st, dt) {
      root.position.set(st.x, st.y, st.z);
      model.rotation.y = st.yaw;
      const c = clipFor({ speed: st.moving ? st.speed ?? 2 : 0, running: st.running, air: !!st.air, vy: st.vy || 0 });
      // no jump clip: a simple tuck instead
      const pick = acts[c.name] ? c.name : st.air ? 'idle' : 'walk';
      play(pick);
      if (acts[pick]) acts[pick].timeScale = c.timeScale;
      model.scale.y = !acts.jump && st.air ? CHAR_SCALE * 0.9 : CHAR_SCALE;
      mixer.update(dt);
    },
    setVisible(v) { root.visible = v; },
    dispose() {
      ctx.stage.scene.remove(root);
      mixer.stopAllAction();
      mixer.uncacheRoot(model);
      model.traverse((o) => { if (o.isSkinnedMesh) o.skeleton?.dispose(); });
      for (const m of own) m.dispose();
      ringGeo.dispose();
      ringMat.dispose();
      ringTex.dispose();
    },
  };
}
