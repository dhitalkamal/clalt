// looks of team members and interns on an agent (agent.js builds them, nothing here knows the garden).
// a team member is dimmed while idle (grey tint, less saturation, a little transparent, monitor off) and lit while
// busy (own colours, monitor on, a glow ring in the member colour under the chair). an intern is the same character
// at 0.8 scale with a cap and a lanyard in the member colour. materials of the character are cloned per agent so
// dimming one never touches the others; disposeLook frees those clones.
import * as THREE from 'three';
import { CHAR_SCALE } from '../agentConst.js';

export const INTERN_SCALE = 0.8;
const IDLE = { desat: 0.75, grey: 0.62, alpha: 0.86, ring: 0.14 };
const BUSY = { desat: 0, grey: 1, alpha: 1, ring: 0.6 };

export const colorNum = (c) => {
  const n = typeof c === 'number' ? c : parseInt(String(c || '').replace('#', ''), 16);
  return Number.isFinite(n) ? n : 0x8a8f9a;
};

// record shapes: team member {role:'team'|kind:'team', color, tint, state}, intern {intern:true|kind:'intern', color, tint}
export const isTeamRec = (s) => !!s && (s.role === 'team' || s.kind === 'team');
export const isInternRec = (s) => !!s && (s.intern === true || s.kind === 'intern');
export const recColor = (s) => colorNum(s.tint != null ? s.tint : s.color);

const DESAT = `
  float dl = dot(gl_FragColor.rgb, vec3(0.299, 0.587, 0.114));
  gl_FragColor.rgb = mix(gl_FragColor.rgb, vec3(dl), uDesat);
  #include <dithering_fragment>`;

// own copies of the character materials with a desaturation uniform
function ownMaterials(a) {
  const mats = [];
  a.model.traverse((o) => {
    if (!o.isMesh) return;
    const m = o.material.clone();
    const uni = { value: 0 };
    m.userData.desat = uni;
    m.onBeforeCompile = (sh) => {
      sh.uniforms.uDesat = uni;
      sh.fragmentShader = 'uniform float uDesat;\n' + sh.fragmentShader.replace('#include <dithering_fragment>', DESAT);
    };
    m.customProgramCacheKey = () => 'team-desat';
    o.material = m;
    mats.push(m);
  });
  return mats;
}

function ringMesh(color) {
  const m = new THREE.Mesh(new THREE.RingGeometry(0.2, 0.31, 32),
    new THREE.MeshBasicMaterial({ color, transparent: true, opacity: 0.2, depthWrite: false, blending: THREE.AdditiveBlending, toneMapped: false, side: THREE.DoubleSide }));
  m.rotation.x = -Math.PI / 2;
  m.position.y = 0.014;
  m.renderOrder = 2;
  return m;
}

// team member setup, once per agent: own materials, glow ring, start in the state the record carries
export function initMemberLook(a) {
  if (a.look) return a.look;
  const color = recColor(a.s);
  const look = { mats: ownMaterials(a), color, ring: ringMesh(color), cur: { ...IDLE }, to: IDLE, seen: null };
  a.seat.add(look.ring);
  a.look = look;
  return look;
}

// the routing side calls this when a member goes busy or idle. the page also calls it by itself when the record's
// state changes, so a call is only needed for a state the record does not carry. an explicit call wins until
// the record's own state changes again.
export function setMemberState(a, state) {
  if (!a || !a.model || !a.seat) return;
  const look = initMemberLook(a);
  look.to = state === 'busy' ? BUSY : IDLE;
  look.state = state === 'busy' ? 'busy' : 'idle';
  look.seen = a.s.state;
}

// per frame: follow the record state, ease the look and set the monitor. called after updateScreen
export function tickMemberLook(a, dt) {
  if (!a.look) {
    initMemberLook(a);
    a.look.cur = { ...(a.s.state === 'busy' ? BUSY : IDLE) };
  }
  const look = a.look;
  if (look.seen !== a.s.state) setMemberState(a, a.s.state);
  const k = 1 - Math.exp(-dt * 5);
  for (const key of Object.keys(look.to)) look.cur[key] += (look.to[key] - look.cur[key]) * k;
  const c = look.cur;
  const tint = new THREE.Color().setScalar(c.grey);
  for (const m of look.mats) {
    m.userData.desat.value = c.desat;
    m.color.copy(tint);
    const clear = c.alpha < 0.995;
    if (m.transparent !== clear) { m.transparent = clear; m.needsUpdate = true; }
    m.opacity = c.alpha;
  }
  const busy = look.state === 'busy';
  a.screenPlane.visible = busy;
  if (!busy) { a.screenGlow.material.opacity = 0; a.deskGlow.material.opacity = 0; }
  look.ring.visible = a.mode !== 'away';
  look.ring.material.opacity = busy ? c.ring + 0.12 * Math.sin(a.now * 3) : c.ring;
}

// a thin bar between two points of the x-y plane, for the lanyard string
function bar(mat, ax, ay, bx, by, z) {
  const len = Math.hypot(bx - ax, by - ay);
  const m = new THREE.Mesh(new THREE.BoxGeometry(len, 0.012, 0.008), mat);
  m.position.set((ax + bx) / 2, (ay + by) / 2, z);
  m.rotation.z = Math.atan2(by - ay, bx - ax);
  return m;
}

// intern: 0.8 scale, cap on the head bone, lanyard with a card on the torso bone, all in the member colour.
// agentRig is the Agent (it carries model and bones). calling it twice does nothing.
export function makeInternLook(agentRig, color) {
  const a = agentRig;
  if (a.internLook) return a.internLook;
  const col = colorNum(color);
  const scale = INTERN_SCALE * CHAR_SCALE;
  a.charScale = scale;
  // the seated body sits lower at a smaller scale, lift it back onto the stool
  a.seatY = 0.4 + 0.176 * (CHAR_SCALE - scale);
  a.model.scale.setScalar(scale);
  const paint = new THREE.MeshStandardMaterial({ color: col, roughness: 0.7 });
  const cardMat = new THREE.MeshStandardMaterial({ color: 0xf4efe3, roughness: 0.8 });
  const cap = new THREE.Group();
  const dome = new THREE.Mesh(new THREE.SphereGeometry(0.225, 16, 8, 0, Math.PI * 2, 0, Math.PI / 2), paint);
  dome.position.set(0, 0.15, 0.03);
  const brim = new THREE.Mesh(new THREE.BoxGeometry(0.3, 0.02, 0.15), paint);
  brim.position.set(0, 0.16, 0.25);
  cap.add(dome, brim);
  a.bones.head.add(cap);
  const lan = new THREE.Group();
  lan.add(bar(paint, -0.07, 0.17, 0, 0.1, 0.125), bar(paint, 0.07, 0.17, 0, 0.1, 0.125));
  const card = new THREE.Mesh(new THREE.BoxGeometry(0.1, 0.13, 0.01), cardMat);
  card.position.set(0, 0.04, 0.13);
  const stripe = new THREE.Mesh(new THREE.BoxGeometry(0.1, 0.03, 0.012), paint);
  stripe.position.set(0, 0.085, 0.13);
  lan.add(card, stripe);
  a.bones.torso.add(lan);
  a.internLook = { mats: [paint, cardMat], parts: [cap, lan] };
  return a.internLook;
}

// frees what this file made for one agent: cloned character materials, intern materials and the cap and lanyard geometry
export function disposeLook(a) {
  for (const l of [a.look, a.internLook]) if (l) for (const m of l.mats) m.dispose();
  if (a.internLook) for (const p of a.internLook.parts) p.traverse((o) => o.geometry && o.geometry.dispose());
  a.look = null;
  a.internLook = null;
}
