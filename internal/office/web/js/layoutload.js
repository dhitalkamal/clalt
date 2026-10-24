// the layout the page shows: what clalt serves at /api/layout, or the default office that ships with the page
async function getJSON(url) {
  const res = await fetch(url, { cache: 'no-store', credentials: 'same-origin' });
  if (!res.ok) throw Object.assign(new Error('http ' + res.status), { status: res.status });
  return res.json();
}

const isV1 = (l) => l && l.v === 1 && l.size && Array.isArray(l.rooms) && Array.isArray(l.furniture);
const isV2 = (l) => l && l.v === 2 && l.site && Array.isArray(l.pavilions);
const isLayout = (l) => isV1(l) || isV2(l);

// resolves to {edit, layout, templates, rev}. template is a template id or omitted for the server default.
// edit is true only when the server says so. throws on any failure, with the http status on e.status when there is one.
export async function loadLayout(template) {
  const body = await getJSON('/api/layout' + (template ? '?template=' + encodeURIComponent(template) : ''));
  // the answer is {edit, layout, templates, rev}; a bare layout object is accepted too
  const bare = isLayout(body);
  const layout = bare ? body : body && body.layout;
  if (!isLayout(layout)) throw new Error('not a layout');
  return {
    edit: bare ? false : body.edit === true,
    layout,
    templates: bare || !Array.isArray(body.templates) ? [] : body.templates,
    rev: bare || body.rev == null ? '' : String(body.rev),
  };
}

// the first load: the wanted template, else the server default, else the hall that ships with the page.
// only here does a failure turn into the hall, later loads keep the scene they have.
export async function loadFirst(want) {
  if (want) {
    try {
      const body = await loadLayout(want);
      if (!body.templates.length || body.templates.some((t) => t.id === want)) return { ...body, template: want };
    } catch (e) { /* a stale or unknown id: try the server default */ }
  }
  try { return { ...(await loadLayout()), template: null }; } catch (e) { /* the server is out of reach */ }
  return { edit: false, layout: await getJSON('./layout/default.json'), templates: [], rev: '', template: null };
}
