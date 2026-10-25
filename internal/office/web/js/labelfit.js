// screen-space de-overlap of the floating labels. pure: boxes in, offsets out.
// an item is {id, x, y, w, h, cw, ch, pri}: it is anchored at its bottom centre (x, y) and covers
// [x - w/2, y - h, x + w/2, y]. cw and ch are the size of its compact one-line form, absent for pills.
// result: Map(id -> {dx, dy, compact, leader, hidden}), offsets in screen px. leader is true when the
// label sits far enough from its anchor to need a line back to it. hidden is only ever set on pills.

const LEADER_AT = 24;
const EPS = 1e-6;

// two boxes closer than gap count as touching
const hit = (a, b, gap) => a.x0 < b.x1 + gap - EPS && b.x0 < a.x1 + gap - EPS && a.y0 < b.y1 + gap - EPS && b.y0 < a.y1 + gap - EPS;

// the box of an item at an offset and size
const boxAt = (it, w, h, dx, dy) => ({ x0: it.x + dx - w / 2, x1: it.x + dx + w / 2, y0: it.y + dy - h, y1: it.y + dy });

const clear = (box, placed, gap) => !placed.some((p) => hit(box, p, gap));

// the smallest move (dx, dy) that clears everything placed, or null when it is longer than cap.
// the best spot always sits against the edge of a placed box (or on the anchor), so the candidates
// are every combination of an edge-aligned dx and an edge-aligned dy
function search(it, w, h, placed, gap, cap) {
  const xs = [0];
  const ys = [0];
  for (const p of placed) {
    xs.push(p.x0 - gap - w / 2 - it.x, p.x1 + gap + w / 2 - it.x);
    ys.push(p.y0 - gap - it.y, p.y1 + gap + h - it.y);
  }
  let best = null;
  for (const dx of xs) {
    for (const dy of ys) {
      const d = Math.hypot(dx, dy);
      if (d > cap || (best && d > best.d + EPS)) continue;
      if (!clear(boxAt(it, w, h, dx, dy), placed, gap)) continue;
      // ties: the one nearest straight up, then left
      if (!best || d < best.d - EPS || dy > best.dy || (dy === best.dy && dx < best.dx)) best = { dx, dy, d };
    }
  }
  return best;
}

// reach: how far a full card may move, in card heights. prev: last frame's result, keeps a layout that
// is still valid from flipping to an equal one when anchors drift by a pixel
export function fitLabels(items, { gap = 3, nudge: reach = 1, prev = null } = {}) {
  // cards before pills, then the lowest on screen first so the front ones stay put
  const order = [...items].sort((a, b) => (b.pri || 0) - (a.pri || 0) || b.y - a.y || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
  const placed = [];
  const out = new Map();
  const done = (it, w, h, pick) => {
    const d = Math.hypot(pick.dx, pick.dy);
    out.set(it.id, { dx: pick.dx, dy: pick.dy, compact: !!pick.compact, leader: d > LEADER_AT, hidden: false });
    placed.push(boxAt(it, w, h, pick.dx, pick.dy));
  };
  for (const it of order) {
    const old = prev && prev.get(it.id);
    // a spot kept from the last frame wins unless a clearly shorter one exists. it may have eased
    // into half the gap, which still reads as clear and saves the label from hopping
    const keep = (w, h, compact, cap) => {
      if (!old || old.hidden || old.compact !== compact) return null;
      const d = Math.hypot(old.dx, old.dy);
      return d <= cap && clear(boxAt(it, w, h, old.dx, old.dy), placed, gap / 2) ? { dx: old.dx, dy: old.dy, d } : null;
    };
    const choose = (w, h, compact, cap) => {
      const best = search(it, w, h, placed, gap, cap);
      const kept = keep(w, h, compact, cap);
      return kept && (!best || kept.d <= best.d + 8) ? { ...kept, compact } : best && { ...best, compact };
    };
    if (it.cw) {
      // a card unfolds again only with some room to spare, so it does not flicker at the edge
      const cap = it.h * reach * (old && old.compact ? 0.7 : 1);
      const full = choose(it.w, it.h, false, cap);
      if (full) { done(it, it.w, it.h, full); continue; }
      // the compact form may go as far as it has to: a card is never hidden
      done(it, it.cw, it.ch, choose(it.cw, it.ch, true, Infinity));
      continue;
    }
    // a pill drifts further than a card, and is hidden when there is no room near its anchor
    const cap = it.h * 4 * (old && old.hidden ? 0.7 : 1);
    const pick = choose(it.w, it.h, false, cap);
    if (pick) done(it, it.w, it.h, pick);
    else out.set(it.id, { dx: 0, dy: 0, compact: false, leader: false, hidden: true });
  }
  return out;
}
