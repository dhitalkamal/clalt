// floating labels of the garden: an account card over each cabin and a pill over every other pavilion, with the
// number of agents in the busy rooms. items follow ui.setLabels: {id, cls, key, render(div), x, y, z}.
import { renderWing } from '../ui.js';
import { toWorld, sizeOf } from './footprints.js';
import { TH_X } from './local.js';

const PILL = { research: 'Research', workspace: 'Open workspace', lobby: 'Lobby', cafe: 'Cafe', play: 'Play' };

// agents per pavilion, and per treehouse side
function counts(L) {
  const n = new Map();
  for (const slot of L.slots.values()) {
    if (!slot.room) continue;
    const k = slot.seatKind === 'review' || slot.seatKind === 'meeting' ? slot.room + ':' + slot.seatKind : slot.room;
    n.set(k, (n.get(k) || 0) + 1);
  }
  return n;
}

function pill(id, text, n, x, y, z) {
  return { id, cls: 'space', key: text + n, x, y, z, render: (d) => {
    d.append(text);
    if (n != null) { const s = document.createElement('span'); s.className = 'n'; s.textContent = String(n); d.append(s); }
  } };
}

export function gardenLabels(L, accounts) {
  const used = counts(L);
  const out = [];
  for (const p of L.layout.pavilions || []) {
    const { d } = sizeOf(p);
    const at = (lx, lz) => toWorld(p, lx, lz);
    if (p.type === 'cabin') {
      const info = accounts.get(p.account) || { label: p.label || p.account, limits: null };
      const [x, z] = at(0, -d / 2);
      out.push({ id: 'w-' + p.id, cls: 'zone', key: JSON.stringify([info.label, info.limits]), x, y: 1.55, z, render: (div) => renderWing(div, info.label, info.limits) });
    } else if (p.type === 'treehouse') {
      const [rx, rz] = at(TH_X - 2.1, 0.4);
      const [mx, mz] = at(TH_X + 2.1, 0.4);
      out.push(pill(p.id + ':review', 'Review', used.get(p.id + ':review') || 0, rx, 3.0, rz), pill(p.id + ':meeting', 'Meeting', used.get(p.id + ':meeting') || 0, mx, 3.0, mz));
    } else {
      const [x, z] = at(0, -d / 2);
      const n = p.type === 'research' || p.type === 'workspace' ? used.get(p.id) || 0 : null;
      out.push(pill(p.id, p.label && p.type !== 'cabin' ? p.label : PILL[p.type] || p.type, n, x, p.type === 'lobby' ? 1.0 : p.type === 'play' ? 1.0 : 1.7, z));
    }
  }
  return out;
}
