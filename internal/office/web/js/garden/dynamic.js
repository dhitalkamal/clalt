// the part of the garden that follows the roster: a spare desk where a desk seat is empty and a spare stool at an
// empty review or meeting seat. an agent brings its own desk or stool, so occupied seats draw nothing here.
import * as THREE from 'three';
import { spareDesk } from '../zones.js';
import { makeStool } from '../props.js';
import { rad } from './geo.js';
import { MEZ } from './local.js';

const EMPTY_TINT = 0x9fb4c7;

// taken: Set of seat ids with an agent. tintFor(account) -> colour
export function buildSpares(L, taken, tintFor) {
  const g = new THREE.Group();
  for (const s of L.seats) {
    if (taken.has(s.id)) continue;
    if (s.kind === 'desk') {
      const d = spareDesk(s.side, s.account ? tintFor(s.account) : EMPTY_TINT);
      d.position.set(s.x, 0, s.z);
      d.rotation.y = rad(s.rot);
      g.add(d);
    } else if (s.kind === 'review' || s.kind === 'meeting') {
      const st = makeStool(EMPTY_TINT, 1);
      st.position.set(s.x, MEZ, s.z);
      g.add(st);
    }
  }
  return g;
}
