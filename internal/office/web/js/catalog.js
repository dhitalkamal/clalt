// the catalog of placeable items: footprints in cells, allowed levels and tags. the page reads the copy that
// ships with it, a clalt test keeps that copy identical to the one the layout validator embeds.
import { failText } from './lostaccess.js';
let cached = null;

const valid = (c) => Array.isArray(c) && c.length > 0 && c.every((x) => x && typeof x.id === 'string' && x.footprint && Array.isArray(x.levels));

async function fetchJSON(url) {
  const res = await fetch(url, { cache: 'no-store', credentials: 'same-origin' });
  if (!res.ok) throw new Error(failText(res.status, ''));
  return res.json();
}

const named = (c) => ({ ...c, tags: c.tags || [], label: c.label || c.id.replace(/-/g, ' '), category: c.category || 'other' });

// resolves to [{id, footprint:{w,d}, levels, tags, label, category}]
export function loadCatalog() {
  if (!cached) {
    cached = (async () => {
      const list = await fetchJSON('./layout/catalog.json');
      if (!valid(list)) throw new Error('the item catalog is not valid');
      return list.map(named);
    })();
  }
  return cached;
}

export const catalogIndex = (list) => new Map(list.map((c) => [c.id, c]));
