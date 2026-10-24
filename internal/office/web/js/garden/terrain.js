// the island: lawn slabs on a stack of soil and rock layers, the water cap when a river runs through, and the
// clouds drifting under it. flat ground, every lawn sits at y = 0 minus DROP so decks stand clear of it.
import * as THREE from 'three';
import { rng, scalePoly, boundsOf } from './geo.js';
import { std, cached, track, geo } from './res.js';
import { grassTex, waterTex } from './tex.js';

export const SKY = 0xbfe4f4;
export const WATER_Y = -0.25;
export const DROP = 0.04;

const toShape = (poly) => new THREE.Shape(poly.map(([x, z]) => new THREE.Vector2(x, -z)));

// extrude a polygon between y0 and y1. materials: [caps, sides]
function slab(poly, y0, y1, mats) {
  const g = track(new THREE.ExtrudeGeometry(toShape(poly), { depth: y1 - y0, bevelEnabled: false, curveSegments: 1 }));
  g.rotateX(-Math.PI / 2);
  g.translate(0, y0, 0);
  return g.attributes.position.count ? new THREE.Mesh(g, mats) : null;
}

// soil and rock layers under a polygon so the chunk reads as a floating island
function rockStack(poly, top, water, ticks) {
  const g = new THREE.Group();
  const soil = std(0x8a6a48);
  const layers = [
    { k: 1.0, d: 0.5, m: soil },
    { k: 0.9, d: 0.9, m: std(0x9b7a55, { flatShading: true }) },
    { k: 0.72, d: 1.1, m: std(0x8d8a84, { flatShading: true }) },
    { k: 0.45, d: 1.5, m: std(0x7d7a76, { flatShading: true }) },
  ];
  let y = top;
  layers.forEach((l, i) => {
    let cap = l.m;
    if (i === 0 && water) {
      const t = waterTex();
      ticks.push((time) => { t.offset.set(time * 0.012, time * 0.006); });
      cap = cached('m:water-cap', () => new THREE.MeshStandardMaterial({ color: 0x45a3cf, map: t, roughness: 0.12, metalness: 0 }));
    }
    const m = slab(scalePoly(poly, l.k), y - l.d, y, [cap, l.m]);
    if (m) { m.receiveShadow = true; m.castShadow = i > 0; g.add(m); }
    y -= l.d;
  });
  return g;
}

// puffy clouds below and around the island, all puffs in one instanced mesh
function clouds(bb) {
  const r = rng(21);
  const mat = cached('m:cloud', () => new THREE.MeshStandardMaterial({ color: 0xffffff, roughness: 1, flatShading: true, emissive: 0xdfeefa, emissiveIntensity: 0.5 }));
  const puff = geo('cloud-puff', () => new THREE.IcosahedronGeometry(1, 1));
  const cx = (bb.x0 + bb.x1) / 2;
  const cz = (bb.z0 + bb.z1) / 2;
  const hx = (bb.x1 - bb.x0) / 2;
  const hz = (bb.z1 - bb.z0) / 2;
  const mats = [];
  const m4 = new THREE.Matrix4();
  for (let i = 0; i < 52; i++) {
    const n = 3 + Math.floor(r() * 3);
    let x;
    let z;
    do { x = (r() - 0.5) * hx * 5; z = (r() - 0.5) * hz * 5.5; } while (Math.abs(x) < hx + 3.5 && Math.abs(z) < hz + 3.5);
    const y = -3.4 - r() * 3.2;
    const sc = 1 + r() * 1.3;
    for (let k = 0; k < n; k++) {
      const rad = 0.7 + r() * 0.7;
      m4.compose(new THREE.Vector3(cx + x + (k * 0.9 - n * 0.45) * sc, y + r() * 0.3 * sc, cz + z + (r() - 0.5) * 0.8 * sc),
        new THREE.Quaternion(), new THREE.Vector3(rad * sc, rad * sc * 0.55, rad * sc));
      mats.push(m4.clone());
    }
  }
  const inst = track(new THREE.InstancedMesh(puff, mat, mats.length));
  mats.forEach((m, i) => inst.setMatrixAt(i, m));
  inst.frustumCulled = false;
  return inst;
}

// builds lawns, the rock stack, water and clouds for a v2 layout
export function buildTerrain(layout) {
  const s = layout.site;
  const g = new THREE.Group();
  const ticks = [];
  const grass = cached('m:grass', () => new THREE.MeshStandardMaterial({ map: grassTex(0.25), roughness: 1 }));
  const water = !!s.river;
  g.add(rockStack(s.base.poly, water ? WATER_Y : -0.3, water, ticks));
  const side = std(0x8a6a48);
  for (const l of s.land) {
    const m = slab(l.poly, -0.3, -DROP, [grass, side]);
    if (!m) continue;
    m.receiveShadow = true;
    m.castShadow = true;
    g.add(m);
  }
  const b = boundsOf(s.base.poly);
  const bb = { ...b, top: 3.4, bottom: -1.2 };
  g.add(clouds(bb));
  return { group: g, ticks, bb };
}
