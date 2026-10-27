// pure text and atlas math of the team nameplates, no three.js so node tests can import it
export const COLS = 4;
export const ROWS = 16;
export const PLATE_CELLS = COLS * ROWS;
const NAME_MAX = 28;

// [{id, name, color}] in members order. a member the list does not know shows its id with a grey accent
export function plateTexts(members, team) {
  const by = new Map((team || []).map((t) => [t.id, t]));
  return (members || []).map((id) => {
    const t = by.get(id);
    return { id, name: String((t && t.name) || id).slice(0, NAME_MAX), color: (t && /^#[0-9a-f]{6}$/i.test(t.color) && t.color) || '#8a8f9a' };
  });
}

// uv rectangle of atlas cell i, v up as three.js expects
export function plateCell(i) {
  const k = Math.max(0, i | 0) % PLATE_CELLS;
  const cx = k % COLS;
  const cy = Math.floor(k / COLS);
  return { u0: cx / COLS, u1: (cx + 1) / COLS, v0: 1 - (cy + 1) / ROWS, v1: 1 - cy / ROWS };
}

