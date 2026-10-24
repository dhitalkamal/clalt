// pure hit tests and placement checks in cell space
import { furnitureRect, pointInRect, rectsOverlap, rectInside, levelBounds, rotateFootprint } from './geometry.js';

// topmost furniture (last in the list) under a cell
export function hitFurniture(layout, catMap, level, cx, cz) {
  for (let i = layout.furniture.length - 1; i >= 0; i--) {
    const f = layout.furniture[i];
    const r = f.level === level ? furnitureRect(f, catMap) : null;
    if (r && pointInRect(cx, cz, r)) return f;
  }
  return null;
}

export function hitRoom(layout, level, cx, cz) {
  for (let i = layout.rooms.length - 1; i >= 0; i--) {
    const r = layout.rooms[i];
    if (r.level === level && pointInRect(cx, cz, r)) return r;
  }
  return null;
}

// top-left cell that puts the footprint center near the cursor cell
export function placeAnchor(catMap, item, rot, cx, cz) {
  const c = catMap.get(item);
  if (!c) return null;
  const fp = rotateFootprint(c.footprint, rot);
  return { x: cx - Math.floor(fp.w / 2), z: cz - Math.floor(fp.d / 2) };
}

// true when the item fits the level and hits no other furniture
export function canPlace(layout, catMap, item, rot, level, x, z, ignoreId) {
  const c = catMap.get(item);
  const bounds = levelBounds(layout, level);
  if (!c || !bounds || !(c.levels || [0]).includes(level)) return false;
  const rect = furnitureRect({ item, x, z, rot }, catMap);
  if (!rectInside(rect, bounds)) return false;
  // floor items (rugs) never collide, either way
  if ((c.tags || []).includes('floor')) return true;
  return !layout.furniture.some((f) => {
    if (f.level !== level || f.id === ignoreId || (catMap.get(f.item)?.tags || []).includes('floor')) return false;
    const o = furnitureRect(f, catMap);
    return o && rectsOverlap(rect, o);
  });
}
