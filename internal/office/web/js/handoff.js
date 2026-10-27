// the session a finished subagent walks to: an intern goes to the session that started it
export const handoffOwner = (rec) => rec.owner || (rec.session && rec.session.id) || null;
