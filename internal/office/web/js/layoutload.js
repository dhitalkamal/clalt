// the layout the page shows: what clalt serves at /api/layout, or the default office that ships with the page
async function getJSON(url) {
  const res = await fetch(url, { cache: 'no-store', credentials: 'same-origin' });
  if (!res.ok) throw new Error('http ' + res.status);
  return res.json();
}

const isLayout = (l) => l && l.v === 1 && l.size && Array.isArray(l.rooms) && Array.isArray(l.furniture);

// resolves to {edit, layout}. edit is true only when the server says so. any failure falls back to the default.
export async function loadLayout() {
  try {
    const body = await getJSON('/api/layout');
    // the answer is {edit, layout}; a bare layout object is accepted too
    const layout = isLayout(body) ? body : body.layout;
    if (isLayout(layout)) return { edit: isLayout(body) ? false : body.edit === true, layout };
  } catch (e) { /* fall through to the embedded default */ }
  return { edit: false, layout: await getJSON('./layout/default.json') };
}
