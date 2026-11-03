// what an agent does while it is idle: stay at the desk, take a break, hide, or rest in a room. pure, no dom.
// rules: {default, teamDefault, accounts: {id: choice}, teams: {id: choice}, subtypes: {type: choice}}
// choice: 'desk' | 'break' | 'hide' | 'room:<roomId>'
export const DEFAULT_RULES = { default: 'break', teamDefault: 'desk', accounts: {}, teams: {}, subtypes: {} };

const FLAT = ['desk', 'break', 'hide'];
const MAPS = ['accounts', 'teams', 'subtypes'];
const valid = (c) => typeof c === 'string' && (FLAT.includes(c) || /^room:.+/.test(c));

// a rules object with every field present and only usable choices
export function normalizeRules(raw) {
  const r = raw && typeof raw === 'object' ? raw : {};
  const out = { default: valid(r.default) ? r.default : DEFAULT_RULES.default, teamDefault: valid(r.teamDefault) ? r.teamDefault : DEFAULT_RULES.teamDefault };
  for (const k of MAPS) {
    out[k] = {};
    const m = r[k];
    if (!m || typeof m !== 'object' || Array.isArray(m)) continue;
    for (const [id, c] of Object.entries(m)) if (valid(c)) out[k][id] = c;
  }
  return out;
}

// scope: 'default' | 'teamDefault' | 'accounts' | 'teams' | 'subtypes'. an empty choice clears the rule. returns a copy
export function withChoice(rules, scope, id, choice) {
  const out = { ...rules, accounts: { ...rules.accounts }, teams: { ...rules.teams }, subtypes: { ...rules.subtypes } };
  if (!MAPS.includes(scope)) { out[scope] = choice; return out; }
  if (choice) out[scope][id] = choice; else delete out[scope][id];
  return out;
}

// the raw choice of the most specific rule that applies
function pick(s, rules) {
  if (s.role === 'team') return rules.teams[s.team] || rules.teamDefault;
  if (s.intern && s.team && rules.teams[s.team]) return rules.teams[s.team];
  if (s.role === 'sub' && rules.subtypes[s.type]) return rules.subtypes[s.type];
  return rules.accounts[s.account] || rules.default;
}

// -> 'desk' | 'break' | 'hide' | {room}. roomIds are the rooms a choice may name (none in a garden)
export function idleChoice(s, rules, roomIds) {
  const c = pick(s, normalizeRules(rules));
  if (c.startsWith('room:')) {
    const id = c.slice(5);
    return (roomIds || []).includes(id) ? { room: id } : 'break';
  }
  return c;
}

// the rooms a choice can name in a layout: [{id, label}], none for a garden or without a layout
export function roomsOf(layout) {
  if (!layout || layout.v === 2) return [];
  return (layout.rooms || []).filter((r) => r.type !== 'lobby' && r.type !== 'cabins').map((r) => ({ id: r.id, label: r.label || r.type || r.id }));
}
