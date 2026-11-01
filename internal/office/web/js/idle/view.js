// the words and lists of the idle controls. pure, no dom.
const FIXED = [['desk', 'Stay at desk'], ['break', 'Take a break (cafe, play area)'], ['hide', 'Hide until it works again']];
const NAMES = { desk: 'stay at desk', break: 'take a break', hide: 'hide until it works again' };

// a choice in a sentence. a room that is not in the layout is shown as what it falls back to
export function choiceName(choice, rooms) {
  if (choice && choice.startsWith('room:')) {
    const r = (rooms || []).find((x) => x.id === choice.slice(5));
    return r ? 'rest in ' + r.label : NAMES.break;
  }
  return NAMES[choice] || NAMES.break;
}

export const customCount = (rules) => Object.keys(rules.accounts).length + Object.keys(rules.teams).length + Object.keys(rules.subtypes).length;

export function summaryText(rules, rooms) {
  const n = customCount(rules);
  const custom = n === 0 ? 'no custom choices' : n + ' custom choice' + (n > 1 ? 's' : '');
  return 'everyone: ' + choiceName(rules.default, rooms) + ', team members: ' + choiceName(rules.teamDefault, rooms) + ', ' + custom;
}

// the select options: [{value, text}]. same is the name the empty option falls back to (none on the two default rows).
// current is the saved value: a room that no longer exists stays in the list so the select shows the truth
export function optionsFor(rooms, same, current) {
  const out = [];
  if (same) out.push({ value: '', text: 'Same as ' + same });
  for (const [value, text] of FIXED) out.push({ value, text });
  for (const r of rooms) out.push({ value: 'room:' + r.id, text: 'Rest in ' + r.label });
  if (current && current.startsWith('room:') && !out.some((o) => o.value === current)) out.push({ value: current, text: 'Rest in a missing room (takes a break)' });
  return out;
}

// what the quick menu edits for an agent record: {scope, id, same, saved, hint}, null for runs that have no rule
export function targetOf(s, accountLabel) {
  if (s.role === 'team' || (s.intern && s.team)) return { scope: 'teams', id: s.team, same: 'all team members', saved: 'saved for this team member', hint: 'applies to all ' + s.team + ' team members' };
  if (s.role === 'sub') return { scope: 'subtypes', id: s.type, same: 'everyone', saved: 'saved for every ' + s.type + ' subagent', hint: 'applies to every ' + s.type + ' subagent' };
  if (s.role === 'main') return { scope: 'accounts', id: s.account, same: 'everyone', saved: 'saved for every session of ' + accountLabel(s.account), hint: 'applies to every session of ' + accountLabel(s.account) };
  return null;
}

// subagent types worth a row: the types running now and the ones with a saved rule
export function subtypesOf(sessions, saved) {
  const set = new Set(Object.keys(saved || {}));
  for (const s of sessions) for (const x of s.subs) if (!x.intern && x.type) set.add(x.type);
  return [...set].sort();
}

// the line under the quick menu: what a change would apply to, then what a save in this panel changed
export const scopeNote = (target, savedHere) => (savedHere ? target.saved : target.hint);
