// signatures: planSeats(layout, roster, catalog?, prev?, garden?) and breakSpots(layout, catalog?, garden?).
// garden is {seats, overflow} for v2 layouts; setGardenSource (routing/garden.js) supplies it when omitted.
// catalog is [{id, footprint, tags}]; without it each furniture item's own tags and footprint are
// used and one console.warn is logged per layout object.
// seat planning for the office page: which agent sits where, and where idle agents take a break.
// pure functions over a layout json (v1, and v2 through routing/garden.js), no dom, no three.js.
import { planGarden, gardenBreakSpots } from './routing/garden.js';
import { badgeCounts } from './routing/team.js';
import { planHall, roomTypeFor } from './routing/hall.js';
import { catalogIndex, spotsWithTags, blockedCells, freeCells } from './routing/spatial.js';

const warned = new WeakSet();
function checkCatalog(catalog, layout) {
  if (catalog?.length || warned.has(layout)) return;
  warned.add(layout);
  console.warn('routing: no catalog given, using the tags on each furniture item');
}

// planSeats(layout, roster, catalog?, prev?) -> Map(agentKey -> {kind, level, x, z, rot, room, seat?})
// roster = [{key, kind: "main"|"team"|"intern"|"sub", account, type, state}]. x and z are cell coordinates
// of the seat center (or the standing cell center), rot is the furniture rotation in degrees.
// working agents get desks before resting ones (routing/hall.js, routing/garden.js); prev is the last
// returned plan so agents that keep their state keep their seat between polls.
export function planSeats(layout, roster, catalog, prev, garden) {
  // v2 layouts plan on the seat list of the garden renderer; garden is {seats, overflow} or comes from setGardenSource
  if (layout.v === 2) return planGarden(layout, roster, prev, garden);
  checkCatalog(catalog, layout);
  return planHall(layout, catalogIndex(catalog), roster, prev);
}

// breakSpots(layout, catalog?) -> {cafe, play, all}. each spot is {kind, level, x, z, rot, room, seat?}.
// spots are tagged furniture (seat, play, break) in cafe and play rooms; a room with none offers
// a few free floor cells; with neither room type the lobby floor is used so idle agents still have
// somewhere to stand.
export function breakSpots(layout, catalog, garden) {
  if (layout.v === 2) return gardenBreakSpots(layout, garden);
  checkCatalog(catalog, layout);
  const by = catalogIndex(catalog);
  const tagged = spotsWithTags(layout, by, ['seat', 'play', 'break']);
  const blocked = blockedCells(layout, by);
  const forType = (type) => {
    const out = [];
    for (const r of (layout.rooms || []).filter((x) => x.type === type)) {
      const mine = tagged.filter((s) => s.room.id === r.id);
      if (mine.length) out.push(...mine.map((s) => ({ kind: type, level: s.level, x: s.x, z: s.z, rot: s.rot, room: r.id, seat: s.id })));
      else out.push(...freeCells(r, blocked, 8).map((c) => ({ kind: type, ...c })));
    }
    return out;
  };
  const cafe = forType('cafe');
  const play = forType('play');
  let all = cafe.concat(play);
  if (!all.length) {
    const lobby = (layout.rooms || []).find((r) => r.type === 'lobby');
    if (lobby) all = freeCells(lobby, blocked, 12).map((c) => ({ kind: 'lobby', ...c }));
  }
  return { cafe, play, all };
}

export { badgeCounts, roomTypeFor };
export { setGardenSource } from './routing/garden.js';
