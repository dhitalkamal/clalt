// the card shown when a room or pavilion is clicked: its name, the account of a cabin and who is inside.
// pure: agents are plain {id, label, x, y, z, away} records, input.js builds them from the live agents.
import { accountLabel } from './tool.js';

const NAMES = { cabin: 'Cabin', research: 'Research', workspace: 'Open workspace', lobby: 'Lobby', cafe: 'Cafe', play: 'Play', treehouse: 'Treehouse', team: 'Team', review: 'Review', meeting: 'Meeting', cabins: 'Cabins' };
const SHOWN = 6;

export function regionInfo(r, layout, accounts, agents) {
  const title = r.label || NAMES[r.type] || r.type || r.id;
  const deck = (r.y || 0) > 0.5;
  const inside = agents.filter((a) => !a.away && a.x > r.x0 && a.x < r.x1 && a.z > r.z0 && a.z < r.z1 && (a.y > 0.5) === deck);
  const rows = [];
  const pav = ((layout && layout.pavilions) || []).find((p) => p.id === r.pavilion);
  if (pav && pav.account) {
    rows.push(['account', accountLabel(accounts, pav.account)]);
  }
  if (pav && pav.type === 'team') {
    const members = agents.filter((a) => a.team);
    rows.push(['members', String(members.length)], ['working', String(members.filter((a) => a.busy).length)]);
  }
  rows.push(['agents here', String(inside.length)]);
  if (inside.length) {
    const names = inside.slice(0, SHOWN).map((a) => a.label);
    if (inside.length > SHOWN) names.push(`+${inside.length - SHOWN} more`);
    rows.push(['inside', names.join(', ')]);
  }
  return { title, rows };
}
