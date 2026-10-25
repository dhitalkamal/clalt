// hover card content for team members and interns. pure: returns {key, title, rows}, ui.js draws it.
// a row value is a string, or {state, text} for a state dot.
export function teamCard(s) {
  if (s.role === 'team') {
    const rows = [['about', s.desc || '-'], ['running', String(s.busy || 0)], ['state', { state: s.state, text: s.state === 'busy' ? 'working' : 'resting' }]];
    if (s.users && s.users.length) rows.push(['used by', s.users.map((u) => `${u.handle} (${u.project})`).join(', ')]);
    return { key: ['t', s.name, s.desc, s.busy, s.state, JSON.stringify(s.users)].join('|'), title: s.name, rows };
  }
  const owner = s.session || {};
  const rows = [['task', s.task || '-'], ['works for', `${owner.handle || s.owner} (${owner.project || '-'})`],
    ['state', { state: 'busy', text: s.leaving ? 'finishing' : 'working' }]];
  return { key: ['i', s.id, s.task, s.leaving].join('|'), title: `${s.type} (intern)`, rows };
}
