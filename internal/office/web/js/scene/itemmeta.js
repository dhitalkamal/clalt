// what the renderer knows about each catalog item beyond its footprint: the solid part (what agents walk
// around), seat rigs and break spots. all local offsets are world units from the footprint centre at rot 0,
// x east, z south (the camera side). items without an entry are fully solid unless they carry a floor or wall tag.
import { SEAT_OFF } from '../layout.js';

const lane = [-0.75, -0.5];
const laneChair = [-0.75, -SEAT_OFF];

export const META = {
  desk: { solid: [1, 1, 3, 2],
    seat: { kind: 'desk', side: 1, rig: [0, 0], approach: lane, inner: [laneChair, [0, -SEAT_OFF]], stand: [laneChair] } },
  cabin: { solid: [0, 0, 4, 4],
    seat: { kind: 'desk', side: -1, rig: [0.25, 0.25], approach: [0.8, 1.25], inner: [[0.8, -0.11], [0.25, -0.11]],
      stand: [[0.8, -0.11], [0.8, 1.3]], cabin: true } },
  'table-seat': { solid: null, seat: { kind: 'round', side: 1, rig: [0, 0], approach: [0, 0], inner: [], stand: [] } },
  'round-table': { solid: [0, 0, 2, 2] },
  'coffee-bar': { solid: [0, 0, 3, 1],
    spots: [{ key: 'a', at: [-0.25, 0.25], act: 'order', face: [-0.25, -0.25], group: 'counter' },
      { key: 'b', at: [0.75, 0.25], act: 'order', face: [0.25, -0.25], group: 'counter' }] },
  'high-table': { solid: [1, 0, 1, 3],
    spots: [{ key: 'a', at: [-0.5, 0], act: 'sip', face: [0, 0], group: 'table', pair: 'b' },
      { key: 'b', at: [0.5, 0], act: 'sip', face: [0, 0], group: 'table', pair: 'a' }] },
  'ping-pong': { solid: [1, 0, 3, 2],
    spots: [{ key: 'a', at: [-1.0, 0.01], act: 'pong', face: [0, 0], group: 'pong', pair: 'b' },
      { key: 'b', at: [1.0, 0.01], act: 'pong', face: [0, 0], group: 'pong', pair: 'a' }] },
  arcade: { solid: [0, 0, 1, 1], spots: [{ key: 'a', at: [0, 0.25], act: 'arcade', face: [0, -0.25], group: 'arcade' }] },
  'water-cooler': { solid: [0, 0, 1, 1], spots: [{ key: 'a', at: [0, 0.25], act: 'cooler', face: [0, -0.25], group: 'cooler' }] },
  'bean-bag-red': { solid: null, spots: [{ key: 'a', at: [0, 0], act: 'read', sit: true, yaw: 0.3, group: 'bag' }] },
  'bean-bag-blue': { solid: null, spots: [{ key: 'a', at: [0, 0], act: 'read', sit: true, yaw: -0.3, group: 'bag' }] },
};

// does the item stop agents: floor decals, hung items and open footprints do not
export function solidRect(def) {
  if (def.tags.includes('floor') || def.tags.includes('wall')) return null;
  if (def.id in META) return META[def.id].solid;
  return [0, 0, def.footprint.w, def.footprint.d];
}
