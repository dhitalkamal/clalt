// screen-space de-overlap of the floating labels. pure: boxes in, offsets out.
// an item is {id, x, y, w, h, cw, ch, pri}: it is anchored at its bottom centre (x, y) and covers
// [x - w/2, y - h, x + w/2, y]. cw and ch are the size of its compact one-line form, absent for pills.
// result: Map(id -> {dy, compact}), dy is a screen offset in px, negative moves up.

const hit = (a, b) => a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1;

// the box of an item at an offset and size
const boxAt = (it, w, h, dy) => ({ x0: it.x - w / 2, x1: it.x + w / 2, y0: it.y + dy - h, y1: it.y + dy });

// moves a box up past whatever it hits, until it is clear or has moved more than cap px.
// returns the offset, or null when it did not fit
function nudge(it, w, h, placed, gap, cap) {
  let dy = 0;
  for (let i = 0; i < 40; i++) {
    const box = boxAt(it, w, h, dy);
    const blocker = placed.filter((p) => hit(box, p)).sort((a, b) => a.y0 - b.y0)[0];
    if (!blocker) return dy;
    dy = blocker.y0 - gap - it.y;
    if (-dy > cap) return null;
  }
  return null;
}

// nudge is the most a full card may move, in card heights
export function fitLabels(items, { gap = 3, nudge: reach = 1 } = {}) {
  // cards before pills, then the lowest on screen first so the front ones stay put
  const order = [...items].sort((a, b) => (b.pri || 0) - (a.pri || 0) || b.y - a.y || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
  const placed = [];
  const out = new Map();
  for (const it of order) {
    let pick = null;
    const full = nudge(it, it.w, it.h, placed, gap, it.h * reach);
    if (full !== null) pick = { dy: full, compact: false };
    else if (it.cw) {
      // a compact card may move as far as a full card is tall: it has to clear the cards it shrank for
      const small = nudge(it, it.cw, it.ch, placed, gap, it.h * 1.5);
      // nothing clear: the small card stays on its anchor
      pick = { dy: small === null ? 0 : small, compact: true };
    } else {
      // a pill may drift further than a card, it is small
      pick = { dy: nudge(it, it.w, it.h, placed, gap, it.h * 6) ?? 0, compact: false };
    }
    out.set(it.id, pick);
    placed.push(boxAt(it, pick.compact ? it.cw : it.w, pick.compact ? it.ch : it.h, pick.dy));
  }
  return out;
}
