// the team pavilion: an open desk pavilion like the workspace with a teal deck, a TEAM sign and a nameplate plus an
// accent strip in the member's colour on each desk. the desks and stools are drawn by the agents and the seat layer
// (dynamic.js), this builder only adds the pavilion shell and the plates. group frame as in pavdesk.js.
import * as THREE from 'three';
import { furniture } from '../assets.js';
import { at, boxGeo, std, shade } from './res.js';
import { textSprite } from './tex.js';
import { deck, pergola } from './parts.js';
import { sizeOf, deskCells, FOOTPRINTS } from './footprints.js';
import { plateTexts } from './teamtext.js';
import { plateMesh, stripMat, teamList } from './teamplates.js';

const TEAL = 0x5fb3a6;
const DESK_TOP = 0.4;
// a desk spans 0.4 in front of its centre, plates sit a little inside that edge
const PLATE_DZ = 0.33;

export function team(p) {
  const { w: W, d: D } = sizeOf(p);
  const f = FOOTPRINTS.team;
  const cols = Math.max(1, p.cols | 0);
  const rows = Math.max(1, p.rows | 0);
  const g = new THREE.Group();
  g.add(deck(W, D, 168, 50, TEAL));
  const cells = deskCells(p);
  const postsX = [];
  for (let c = 2; c < cols; c += 2) postsX.push(cells[c].lx - 1.05);
  g.add(pergola(-W / 2 + 0.04, -D / 2 + 0.04, W / 2 - 0.04, D / 2 - 0.04, 1.55, { wood: 0x4f8f86, postsX }));
  for (let r = 0; r < rows - 1; r++) {
    const hedge = new THREE.Mesh(boxGeo(W - 1.0, 0.18, 0.18), std(0x5aa24f));
    hedge.position.set(0, 0.09, -D / 2 + f.pad / 2 + f.row * (r + 1));
    g.add(shade(hedge));
  }
  g.add(at(textSprite('TEAM', TEAL, 0.6), 0, 1.38, D / 2 - 0.08));
  g.add(at(furniture('pottedPlant', { scale: 1.3 }), -W / 2 + 0.25, 0, D / 2 - 0.25), at(furniture('pottedPlant', { scale: 1.3 }), W / 2 - 0.25, 0, D / 2 - 0.25));
  // one plate and strip per member desk, in members order
  const texts = plateTexts(p.members, teamList());
  texts.slice(0, cells.length).forEach((t, i) => {
    const c = cells[i];
    const dz = c.line ? -1 : 1;
    const holder = new THREE.Group();
    holder.position.set(c.lx, DESK_TOP, c.lz + dz * PLATE_DZ);
    holder.rotation.y = c.line ? Math.PI : 0;
    const plate = plateMesh(t.id);
    plate.position.y = 0.05;
    plate.rotation.x = -0.8;
    const strip = new THREE.Mesh(boxGeo(0.5, 0.012, 0.035), stripMat(t.id, t.color));
    strip.position.set(0, 0.006, 0.05);
    holder.add(plate, strip);
    g.add(holder);
  });
  return g;
}
