// the part of the scene that follows the roster: cabins around their sessions, spare desks and stools for empty
// seats, and the account floor under each group of cabins. rebuilt on every roster change.
import * as THREE from 'three';
import { makeStool } from '../props.js';
import { yawOf, rotate } from './geom.js';
import { cabin, spareDesk, flat } from '../zones.js';

const EMPTY_TINT = 0x9fb4c7;

// occ: Map(item id -> session or sub record). tintFor(account) -> colour.
export function buildDynamic(L, occ, tintFor) {
  const g = new THREE.Group();
  const sway = [];
  const wings = new Map();
  const cabins = L.items.filter((i) => i.def.id === 'cabin');
  const neighbour = (it, lx) => {
    const d = rotate(lx, 0, it.rot);
    return cabins.some((o) => o !== it && o.level === it.level && o.rot === it.rot && Math.abs(o.cx - (it.cx + d.x)) < 0.01 && Math.abs(o.cz - (it.cz + d.z)) < 0.01);
  };
  for (const it of L.items) {
    if (!it.def.tags.includes('seat')) continue;
    const s = occ.get(it.id) || null;
    const holder = new THREE.Group();
    holder.position.set(it.cx, it.y, it.cz);
    holder.rotation.y = yawOf(it.rot);
    if (it.def.id === 'cabin') {
      const tint = s ? tintFor(s.account) : EMPTY_TINT;
      const c = cabin(s, tint, neighbour(it, -2.0), neighbour(it, 2.0));
      holder.add(c.root);
      sway.push({ obj: c.plant, phase: s ? s.seed : it.rec.x });
      if (s) {
        const key = (it.room ? it.room.id : '-') + '|' + s.account;
        const w = wings.get(key) || { room: it.room, account: s.account, x0: Infinity, x1: -Infinity, z0: Infinity, z1: -Infinity };
        w.x0 = Math.min(w.x0, it.cx - it.fw * 0.25); w.x1 = Math.max(w.x1, it.cx + it.fw * 0.25);
        w.z0 = Math.min(w.z0, it.cz - it.fd * 0.25); w.z1 = Math.max(w.z1, it.cz + it.fd * 0.25);
        wings.set(key, w);
      }
    } else if (!s && it.def.id === 'desk') holder.add(spareDesk(1));
    else if (!s && it.def.id === 'table-seat') holder.add(makeStool(EMPTY_TINT, 1));
    else continue;
    g.add(holder);
  }
  for (const w of wings.values()) {
    const f = flat(w.x1 - w.x0 + 0.1, w.z1 - w.z0 + 0.1, tintFor(w.account), 0.3);
    f.position.set((w.x0 + w.x1) / 2, 0.006, (w.z0 + w.z1) / 2);
    g.add(f);
  }
  return { group: g, sway, wings: [...wings.values()] };
}
