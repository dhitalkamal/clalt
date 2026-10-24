// live validation: every rule of the layout contract, plus seat warnings.
// pure, no dom. issue = {sev: 'error'|'warn', code, msg, id?}.
import { furnitureRect, rectsOverlap, rectInside, levelBounds, rectCenterCell, pointInRect, doorSideLength, stairsRects } from './geometry.js';

export const MAX_ROOMS = 64;
export const MAX_FURNITURE = 1500;
const ID_RE = /^[a-z0-9-]{1,32}$/;
const COLOR_RE = /^#[0-9a-fA-F]{6}$/;
const NAME_RE = /^[a-z0-9-]{1,40}$/;
const ACCOUNT_RE = /^[A-Za-z0-9._-]{1,64}$/;
const TYPES = ['lobby', 'cabins', 'research', 'workspace', 'review', 'meeting', 'cafe', 'play'];
const SIDES = ['left', 'right', 'back', 'front'];
const ROTS = [0, 90, 180, 270];

// free text rules of the Go validator: already clean, no url, short enough.
// returns '' when fine, else a short reason
export function textProblem(s, max) {
  const spaced = s.replace(/[\p{Cc}\p{Cf}\p{Zl}\p{Zp}]/gu, ' ');
  if (spaced.split(/\s+/).filter(Boolean).join(' ') !== s) return 'has control characters or odd spacing';
  const low = s.toLowerCase();
  if (['://', 'www.', 'javascript:', 'data:', 'vbscript:'].some((b) => low.includes(b)) || s.startsWith('//')) {
    return 'looks like a url, which a layout never holds';
  }
  if (Array.from(s).length > max) return 'is longer than ' + max + ' characters';
  return '';
}

// how many seat-tagged furniture items sit inside each room
export function seatCounts(layout, catalog) {
  const cat = new Map(catalog.map((c) => [c.id, c]));
  const out = new Map(layout.rooms.map((r) => [r.id, 0]));
  for (const f of layout.furniture) {
    const c = cat.get(f.item);
    const rect = furnitureRect(f, cat);
    if (!c || !rect || !(c.tags || []).includes('seat')) continue;
    const mid = rectCenterCell(rect);
    const room = layout.rooms.find((r) => r.level === f.level && pointInRect(mid.x, mid.z, r));
    if (room) out.set(room.id, out.get(room.id) + 1);
  }
  return out;
}

// opts.sessionCount: number or (room) => number, current sessions for warnings
export function validateLayout(layout, catalog, opts = {}) {
  const cat = new Map(catalog.map((c) => [c.id, c]));
  const issues = [];
  const add = (sev, code, msg, id) => issues.push({ sev, code, msg, id });
  const err = (code, msg, id) => add('error', code, msg, id);

  checkLayoutFields(layout, err);
  checkIds(layout, err);
  checkRooms(layout, err);
  checkFurniture(layout, cat, err);
  checkSeats(layout, catalog, opts.sessionCount, add);

  const errors = issues.filter((i) => i.sev === 'error');
  const warnings = issues.filter((i) => i.sev === 'warn');
  const bad = new Set(errors.filter((i) => i.id).map((i) => i.id));
  const warn = new Set(warnings.filter((i) => i.id).map((i) => i.id));
  return { issues, errors, warnings, bad, warn };
}

function checkLayoutFields(l, err) {
  const { w, d } = l.size || {};
  if (l.v !== 1) err('version', 'Unsupported layout version ' + l.v + '.');
  if (!NAME_RE.test(l.name || '')) err('name-bad', 'Name must be 1 to 40 characters of a-z, 0-9 and dash.');
  if (l.grid !== 0.5) err('grid-bad', 'Grid must be 0.5.');
  if (!(w >= 8 && w <= 120 && d >= 8 && d <= 120)) err('size-range', 'Office size must be 8 to 120 cells on each side.');
  if (!COLOR_RE.test(l.walls?.color || '')) err('color-bad', 'Wall colour must be like #e9e4dc.');
  if (!COLOR_RE.test(l.floorColor || '')) err('color-bad', 'Floor colour must be like #d9b88f.');
  const tp = textProblem(l.title || '', 60);
  if (tp) err(tp.startsWith('is longer') ? 'title-long' : 'text-bad', 'Title ' + tp + '.');
  const door = l.door;
  if (!door || !SIDES.includes(door.side)) err('door-range', 'Door needs a side: left, right, back or front.');
  else if (!Number.isInteger(door.at) || door.at < 0 || door.at >= doorSideLength(l, door.side)) {
    err('door-range', 'Door position is off the ' + door.side + ' wall.');
  }
  checkDeck(l, err);
}

function checkDeck(l, err) {
  const floor = { x: 0, z: 0, w: l.size.w, d: l.size.d };
  if (!l.deck) {
    if (l.stairs) err('stairs-nodeck', 'Stairs are set but there is no deck.');
    return;
  }
  if (!rectInside(l.deck, floor)) err('deck-outside', 'The upper deck must fit inside the office floor.');
  if (!l.stairs) {
    err('stairs-missing', 'A deck needs stairs.');
    return;
  }
  if (!['n', 's', 'e', 'w'].includes(l.stairs.dir)) {
    err('stairs-dir', 'Stairs direction must be n, s, e or w.');
    return;
  }
  const { flight, landing } = stairsRects(l.stairs);
  if (!rectInside(flight, floor) || !rectInside(landing, floor)) {
    err('stairs-outside', 'Stairs (flight and landing) are not inside the office floor.');
  }
}

function checkIds(l, err) {
  for (const [list, noun, max, maxCode] of [
    [l.rooms, 'room', MAX_ROOMS, 'rooms-max'],
    [l.furniture, 'furniture', MAX_FURNITURE, 'furn-max'],
  ]) {
    const seen = new Set();
    if (list.length > max) err(maxCode, 'At most ' + max + ' ' + noun + ' entries (now ' + list.length + ').');
    for (const x of list) {
      if (!ID_RE.test(x.id || '')) err('id-bad', 'Bad ' + noun + ' id "' + x.id + '" (a-z, 0-9, dash, up to 32).', x.id);
      else if (seen.has(x.id)) err('id-dup', 'Duplicate ' + noun + ' id "' + x.id + '".', x.id);
      seen.add(x.id);
    }
  }
}

function checkRooms(l, err) {
  const lobbies = l.rooms.filter((r) => r.type === 'lobby');
  if (!lobbies.some((r) => r.level === 0)) err('lobby-missing', 'The office needs a lobby on the ground floor.');
  if (lobbies.filter((r) => r.level === 0).length > 1) {
    for (const r of lobbies) err('lobby-many', 'Only one lobby is allowed.', r.id);
  }
  for (const r of lobbies) if (r.level !== 0) err('lobby-level', 'The lobby must be on the ground floor.', r.id);

  for (const r of l.rooms) {
    if (!TYPES.includes(r.type)) err('room-type', 'Unknown room type "' + r.type + '".', r.id);
    const rp = textProblem(r.label || '', 40);
    if (rp) err('text-bad', 'Room label ' + rp + '.', r.id);
    if (r.account && r.type === 'cabins' && !ACCOUNT_RE.test(r.account)) err('account-bad', 'Account is not an account id.', r.id);
    const bounds = levelBounds(l, r.level);
    if (!bounds) err('room-outside', r.label + ' is on level ' + r.level + ', which this layout does not have.', r.id);
    else if (!rectInside(r, bounds)) {
      err('room-outside', r.label + ' sticks out of the ' + (r.level ? 'deck' : 'office floor') + '.', r.id);
    }
    if (r.account && r.type !== 'cabins') err('account-type', 'Only cabins can be tied to an account.', r.id);
    if (r.floor && !COLOR_RE.test(r.floor)) err('color-bad', r.label + ' floor colour must be like #aabbcc.', r.id);
  }
  for (let i = 0; i < l.rooms.length; i++) {
    for (let j = i + 1; j < l.rooms.length; j++) {
      const a = l.rooms[i];
      const b = l.rooms[j];
      if (a.level === b.level && rectsOverlap(a, b)) {
        err('room-overlap', a.label + ' overlaps ' + b.label + '.', a.id);
        err('room-overlap', b.label + ' overlaps ' + a.label + '.', b.id);
      }
    }
  }
}

function checkFurniture(l, cat, err) {
  const cells = new Map();
  const pairs = new Set();
  const rooms = new Map(l.rooms.map((r) => [r.id, r]));
  for (const f of l.furniture) {
    const c = cat.get(f.item);
    if (!c) {
      err('furn-unknown', 'Unknown item "' + f.item + '".', f.id);
      continue;
    }
    if (!ROTS.includes(f.rot)) err('furn-rot', c.label + ' rotation must be 0, 90, 180 or 270.', f.id);
    if (!(c.levels || [0]).includes(f.level)) err('furn-level', c.label + ' cannot go on level ' + f.level + '.', f.id);
    if (f.room && (!rooms.has(f.room) || rooms.get(f.room).level !== f.level)) {
      err('furn-room', c.label + ' points at a room that does not exist on its level.', f.id);
    }
    const rect = furnitureRect(f, cat);
    const bounds = levelBounds(l, f.level);
    if (!bounds || !rectInside(rect, bounds)) {
      err('furn-outside', c.label + ' is outside the ' + (f.level ? 'deck' : 'office floor') + '.', f.id);
      continue;
    }
    // rugs and mats lie under other pieces, so they never collide
    if ((c.tags || []).includes('floor')) continue;
    for (let x = rect.x; x < rect.x + rect.w; x++) {
      for (let z = rect.z; z < rect.z + rect.d; z++) {
        const key = f.level + ',' + x + ',' + z;
        const other = cells.get(key);
        if (other && other !== f.id) {
          const pair = other < f.id ? other + '|' + f.id : f.id + '|' + other;
          if (!pairs.has(pair)) {
            pairs.add(pair);
            err('furn-overlap', c.label + ' overlaps another item.', f.id);
            err('furn-overlap', 'An item overlaps ' + c.label + '.', other);
          }
        } else cells.set(key, f.id);
      }
    }
  }
}

function checkSeats(l, catalog, sessionCount, add) {
  if (sessionCount == null) return;
  const seats = seatCounts(l, catalog);
  for (const r of l.rooms) {
    if (r.type !== 'cabins') continue;
    const need = typeof sessionCount === 'function' ? sessionCount(r) : sessionCount;
    if (need > seats.get(r.id)) {
      add('warn', 'seats-short', r.label + ' has ' + seats.get(r.id) + ' seats for ' + need + ' sessions.', r.id);
    }
  }
}
