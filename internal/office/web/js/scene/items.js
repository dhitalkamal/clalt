// places catalog items in the scene: one builder per item id, wrapped in a holder at the footprint centre
import * as THREE from 'three';
import { yawOf } from './geom.js';
import { BUILD_A } from './itemsa.js';
import { BUILD_B } from './itemsb.js';

const BUILDERS = { ...BUILD_A, ...BUILD_B };

// what builders hand back to the app: animation ticks, plants that sway, lamps that glow, printers, ping-pong balls
export const makeBuilder = () => ({ ticks: [], sway: [], lamps: [], printers: [], pongs: [] });

// seat items are drawn by the dynamic layer, the rest here. returns null for an id without a builder.
export function placeItem(it, b, hot) {
  const make = BUILDERS[it.def.id];
  if (!make) return null;
  const obj = make({ b, hot, id: it.id, n: b.sway.length, it });
  const holder = new THREE.Group();
  holder.add(obj);
  holder.position.set(it.cx, it.y, it.cz);
  holder.rotation.y = yawOf(it.rot);
  holder.userData.itemId = it.id;
  return holder;
}

export const hasBuilder = (id) => id in BUILDERS;
