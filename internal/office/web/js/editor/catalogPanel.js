// furniture catalog: search, categories, click an item to start placing it
import { h, selectInput } from './dom.js';

export function createCatalogPanel(ed) {
  const items = ed.catalog.filter((c) => (c.levels || [0]).includes(0));
  const cats = [...new Set(items.map((c) => c.category || 'other'))].sort();
  let query = '';
  let cat = '';
  const search = h('input', {
    type: 'search', placeholder: 'Search furniture', 'aria-label': 'Search furniture',
    oninput: (e) => {
      query = e.target.value.trim().toLowerCase();
      renderList();
    },
  });
  const catSel = selectInput([['', 'All categories'], ...cats.map((c) => [c, c])], '', (v) => {
    cat = v;
    renderList();
  });
  catSel.setAttribute('aria-label', 'Category');
  const list = h('div', { class: 'ed-catalog', role: 'list' });
  const hint = h('p', { class: 'ed-note', text: 'Click a spot on the floor to place. r rotates, Esc stops.' });
  const el = h('section', { class: 'ed-card', 'aria-label': 'Furniture catalog' },
    h('h3', { text: 'Furniture' }), search, catSel, hint, list);

  function renderList() {
    const shown = items.filter((c) => {
      const hay = (c.id + ' ' + (c.label || '') + ' ' + (c.tags || []).join(' ')).toLowerCase();
      return (!cat || (c.category || 'other') === cat) && (!query || hay.includes(query));
    });
    list.replaceChildren(...shown.map((c) => h('button', {
      type: 'button', role: 'listitem', 'aria-pressed': String(ed.furnItem === c.id),
      title: c.id + ' ' + c.footprint.w + ' by ' + c.footprint.d + ' cells',
      onclick: () => {
        ed.furnItem = c.id;
        ed.ghostRot = 0;
        ed.select(null);
        ed.setTool('furniture');
      },
    }, h('span', { text: c.label || c.id }), h('small', { text: c.footprint.w + 'x' + c.footprint.d }))));
    if (!shown.length) list.replaceChildren(h('p', { class: 'ed-note', text: 'No match.' }));
  }

  const refresh = () => {
    el.hidden = ed.tool !== 'furniture';
    if (!el.hidden) renderList();
  };
  ed.on('tool', refresh);
  renderList();
  refresh();
  return { el, search, refresh };
}
