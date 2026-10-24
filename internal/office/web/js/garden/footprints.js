// pavilion sizes and frames. the table is a copy of internal/office/layout/data/footprints.json, which the go
// overlap check reads too. a pavilion is centred on its x,z, rot is in degrees counterclockwise seen from
// above, 0 faces +z (south). local frame: x east, z south, origin at the footprint centre.
import { footprint, rad } from './geo.js';

export const FOOTPRINTS = {
  lobby: { w: 3.8, d: 4.0 }, cafe: { w: 5.6, d: 4.0 }, play: { w: 5.0, d: 4.0 }, treehouse: { w: 9.6, d: 4.2 },
  cabin: { col: 2.1, row: 2.8, pad: 0.0 }, research: { col: 2.1, row: 3.5, pad: 0.2 }, workspace: { col: 2.1, row: 3.5, pad: 0.2 },
};

export const DESK_TYPES = new Set(['cabin', 'research', 'workspace']);

// cabins hold one desk line per row, a little forward of the cell centre. research and workspace rows hold two
// lines facing each other, PAIR_GAP either side of the row centre
const CABIN_DZ = 0.1;
const PAIR_GAP = 0.55;
export const PAIRED = new Set(['research', 'workspace']);

// {w, d} before rotation, or null for an unknown type
export function sizeOf(p) {
  const f = FOOTPRINTS[p.type];
  if (!f) return null;
  if (f.col) {
    const cols = Math.max(1, p.cols | 0);
    const rows = Math.max(1, p.rows | 0);
    return { w: f.col * cols + f.pad, d: f.row * rows + f.pad };
  }
  return { w: f.w, d: f.d };
}

// local point to world, rot in degrees
export function toWorld(p, lx, lz) {
  const t = rad(p.rot || 0);
  const c = Math.cos(t);
  const s = Math.sin(t);
  return [p.x + lx * c + lz * s, p.z - lx * s + lz * c];
}

export function polyOf(p) {
  const s = sizeOf(p);
  return footprint(p.x, p.z, s.w, s.d, rad(p.rot || 0));
}

// one cell per desk: col, row and line (0 north, 1 south) with the desk centre in the local frame. the south line
// of a paired row faces north, so its desks turn by 180 degrees
export function deskCells(p) {
  const f = FOOTPRINTS[p.type];
  const s = sizeOf(p);
  const cols = Math.max(1, p.cols | 0);
  const rows = Math.max(1, p.rows | 0);
  const out = [];
  const lines = PAIRED.has(p.type) ? [0, 1] : [0];
  for (let r = 0; r < rows; r++) {
    const centre = -s.d / 2 + f.pad / 2 + f.row * (r + 0.5);
    for (const line of lines) {
      for (let c = 0; c < cols; c++) {
        const lz = lines.length === 2 ? centre + (line ? PAIR_GAP : -PAIR_GAP) : centre + CABIN_DZ;
        out.push({ col: c, row: r, line, cols, rows, lx: -s.w / 2 + f.pad / 2 + f.col * (c + 0.5), lz });
      }
    }
  }
  return out;
}
