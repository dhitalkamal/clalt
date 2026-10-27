// the three desk pavilions: account cabin, research and open workspace. the desks themselves are not drawn
// here: an agent brings its own desk, and the seat layer draws a spare one where nobody sits (see dynamic.js).
// every builder returns a group in the pavilion frame (origin at the footprint centre, deck top at y = 0).
import * as THREE from 'three';
import { furniture } from '../assets.js';
import { box, at, boxGeo, std, shade, geo, cached, basic } from './res.js';
import { textSprite, whiteboardTex } from './tex.js';
import { flowerBed } from './propmesh.js';
import { deck, awning, pergola, pickets, post, WOOD } from './parts.js';
import { sizeOf, deskCells, FOOTPRINTS } from './footprints.js';

function whiteboard(seed) {
  const g = new THREE.Group();
  g.add(box(0.54, 0.35, 0.012, 0x8a8f96));
  const f = new THREE.Mesh(geo('board-face', () => new THREE.PlaneGeometry(0.5, 0.31)), cached(`m:board:${seed % 3}`, () => new THREE.MeshBasicMaterial({ map: whiteboardTex(seed), toneMapped: false })));
  f.position.z = 0.008;
  g.add(f);
  return g;
}

// account cabin: shared deck, back wall panel with awning, picket fence with a door gap in front
export function cabin(p, tint) {
  const { w: W, d: D } = sizeOf(p);
  const g = new THREE.Group();
  g.add(deck(W, D, 32, 54, tint));
  const zb = -D / 2 + 0.06;
  const zf = D / 2 - 0.05;
  g.add(at(box(W, 0.95, 0.03, 0xc99a62), 0, 0.475, zb));
  g.add(at(box(W, 0.04, 0.06, WOOD), 0, 0.97, zb));
  g.add(at(flowerBed(W - 0.2, 0.2, W * 3), 0, 0, zb - 0.2));
  g.add(at(whiteboard((p.account || 'x').length), -W / 2 + 1.2, 0.62, zb + 0.02));
  g.add(at(box(0.3, 0.3, 0.01, 0xe7e1d5), W / 2 - 1.2, 0.62, zb + 0.02));
  const glass = new THREE.Mesh(geo('window-glass', () => new THREE.PlaneGeometry(0.26, 0.26)), basic(0xbfe0f7));
  g.add(at(glass, W / 2 - 1.2, 0.62, zb + 0.03));
  g.add(pickets(-W / 2, zb, -W / 2, zf), pickets(W / 2, zb, W / 2, zf), pickets(-W / 2, zf, W / 2, zf, -0.3, 0.3));
  for (const x of [-0.3, 0.3]) g.add(at(post(0.95, 0.06), x, 0, zf));
  g.add(at(box(0.7, 0.05, 0.06, WOOD), 0, 0.95, zf));
  g.add(at(textSprite((p.label || p.account || 'cabin').toUpperCase().slice(0, 14), tint, 0.55), 0, 1.12, zf + 0.03));
  for (const x of [-W / 2 + 0.03, W / 2 - 0.03]) g.add(at(post(1.3), x, 0, zb));
  g.add(at(awning(W - 0.04, 0.75, tint), 0, 1.25, zb + 0.28));
  g.add(at(furniture('pottedPlant', { scale: 1.1 }), W / 2 - 0.35, 0, zf - 0.35));
  return g;
}

// open desk pavilion. research adds bookcases along the back with a doorway, workspace adds hedge planters
export function space(p) {
  const { w: W, d: D } = sizeOf(p);
  const research = p.type === 'research';
  const tint = research ? 0x5b8fc9 : 0xd9b26a;
  const cols = Math.max(1, p.cols | 0);
  const rows = Math.max(1, p.rows | 0);
  const g = new THREE.Group();
  g.add(deck(W, D, research ? 30 : 34, research ? 58 : 50, tint));
  // a roof beam post every two columns so a wide pavilion does not sag
  const cells = deskCells(p);
  const postsX = [];
  for (let c = 2; c < cols; c += 2) postsX.push(cells[c].lx - 1.05);
  g.add(pergola(-W / 2 + 0.04, -D / 2 + 0.04, W / 2 - 0.04, D / 2 - 0.04, 1.55, { wood: research ? 0x8b6a44 : 0x9a6a3e, postsX }));
  if (research) {
    const n = Math.floor((W - 0.6) / 0.62);
    for (let i = 0; i < n; i++) {
      const x = -W / 2 + 0.55 + i * 0.62;
      if (Math.abs(x) < 0.5) continue;
      g.add(at(furniture(i % 3 === 1 ? 'bookcaseClosed' : 'bookcaseOpen', { scale: 1.5 }), x, 0, -D / 2 + 0.16));
    }
    g.add(at(furniture('pottedPlant', { scale: 1.3 }), W / 2 - 0.3, 0, D / 2 - 0.3));
  } else {
    // low hedge between the desk rows and a plant at each front corner
    for (let r = 0; r < rows - 1; r++) {
      const hedge = new THREE.Mesh(boxGeo(W - 1.0, 0.18, 0.18), std(0x5aa24f));
      hedge.position.set(0, 0.09, -D / 2 + FOOTPRINTS[p.type].pad / 2 + FOOTPRINTS[p.type].row * (r + 1));
      g.add(shade(hedge));
    }
    g.add(at(furniture('pottedPlant', { scale: 1.3 }), -W / 2 + 0.25, 0, D / 2 - 0.25), at(furniture('pottedPlant', { scale: 1.3 }), W / 2 - 0.25, 0, D / 2 - 0.25));
  }
  return g;
}
