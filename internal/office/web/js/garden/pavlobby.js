// the gate plaza: welcome arch at the back, fountain, reception kiosk. agents arrive under the arch.
import * as THREE from 'three';
import { furniture } from '../assets.js';
import { std, box, at, boxGeo } from './res.js';
import { textSprite } from './tex.js';
import { fountain, lampPost } from './propmesh.js';
import { plaza, awning, post } from './parts.js';
import { sizeOf } from './footprints.js';

export function lobby(p) {
  const { w: W, d: D } = sizeOf(p);
  const g = new THREE.Group();
  g.add(plaza(W, D));
  const wood = std(0x8b5e3c);
  for (const dx of [-0.65, 0.65]) {
    const m = new THREE.Mesh(boxGeo(0.1, 1.25, 0.1), wood);
    m.position.set(dx, 0.625, -D / 2 + 0.2);
    m.castShadow = true;
    g.add(m);
  }
  const top = new THREE.Mesh(boxGeo(1.4, 0.1, 0.12), wood);
  top.position.set(0, 1.28, -D / 2 + 0.2);
  g.add(top);
  g.add(at(textSprite('WELCOME', 0x6aa84f, 0.8), 0, 1.05, -D / 2 + 0.25));
  g.add(at(furniture('rugDoormat', { scale: 1.2, noCast: true }), 0, 0.012, -D / 2 + 0.7));
  g.add(at(fountain(), -0.95, 0, -0.1));
  // reception: desk with a screen under a small kiosk roof on four posts
  const desk = new THREE.Group();
  desk.add(at(box(1.1, 0.4, 0.5, 0xe8dcc6), 0, 0.2, 0));
  desk.add(at(box(1.14, 0.03, 0.54, 0x8a5a36), 0, 0.415, 0));
  desk.add(at(furniture('computerScreen', { scale: 0.9 }), 0.25, 0.43, -0.1));
  g.add(at(desk, 1.0, 0, 0.7));
  for (const [x, z] of [[0.35, 0.25], [0.35, 1.15], [1.65, 0.25], [1.65, 1.15]]) g.add(at(post(1.2, 0.06), x, 0, z));
  g.add(at(awning(1.6, 1.1, 0x6aa84f, 0), 1.0, 1.2, 0.7));
  g.add(at(lampPost(), -1.6, 0, -D / 2 + 0.3), at(lampPost(), 1.6, 0, -D / 2 + 0.3));
  g.add(at(furniture('pottedPlant', { scale: 1.4 }), -1.55, 0, D / 2 - 0.3));
  return g;
}
