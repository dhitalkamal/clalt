// which log a clicked character opens. pure: records are the roster records (see roster.js and model.js).
// a main session opens its conversation, a subagent or intern its own run, a team member the run it stands for.
export const agentOfSub = (sub) => {
  const prefix = (sub.session ? sub.session.handle : '') + '/';
  return sub.id.startsWith(prefix) && prefix !== '/' ? sub.id.slice(prefix.length) : '';
};

// returns {handle, agent, kind} or null when the character has no run to show
export function activityTarget(rec, sessions) {
  if (rec.role === 'review') return { handle: rec.handle, agent: '', kind: 'session' };
  if (rec.role === 'main') return { handle: rec.handle, agent: '', kind: 'session' };
  if (rec.role === 'team') {
    let best = null;
    for (const s of sessions) {
      if (s.leaving) continue;
      for (const x of s.subs) {
        if (x.leaving || x.team !== rec.team) continue;
        const score = (x.mapped ? 1e15 : 0) + (x.startedAt || 0);
        if (!best || score > best.score) best = { score, handle: s.handle, sub: x };
      }
    }
    const agent = best ? agentOfSub({ id: best.sub.id, session: { handle: best.handle } }) : '';
    return agent ? { handle: best.handle, agent, kind: 'run' } : null;
  }
  const agent = agentOfSub(rec);
  return agent && rec.session ? { handle: rec.session.handle, agent, kind: 'run' } : null;
}

// header fields for a record: {title, account, project, state, stateText}. accounts is the label map from the snapshot.
export function describeTarget(rec, accounts, sessions) {
  const label = (id) => (accounts && accounts.get(id) ? accounts.get(id).label : id || '-');
  if (rec.role === 'review') return { title: rec.title, account: label(rec.account), project: rec.project || '-', state: rec.state, stateText: 'review run' };
  if (rec.role === 'main') {
    return { title: 'claude (main) ' + rec.handle, account: label(rec.account), project: rec.project || '-', state: rec.state, stateText: rec.state };
  }
  if (rec.role === 'team') {
    const user = (sessions || []).find((s) => !s.leaving && s.subs.some((x) => !x.leaving && x.team === rec.team));
    return { title: rec.name + ' (team)', account: label(user ? user.account : ''), project: user ? user.project : '-', state: rec.state, stateText: rec.state === 'busy' ? 'working' : 'resting' };
  }
  const s = rec.session || {};
  return { title: rec.type + (rec.intern ? ' (intern)' : ''), account: label(rec.account), project: s.project || '-', state: 'busy', stateText: rec.leaving ? 'finishing' : 'working' };
}
