// keeps the editor in step with the server after the active layout was deleted: it loads the fallback layout
// (or closes when the editor cannot edit it), so a Save cannot write the deleted layout back.
export async function reloadEditorAfterDelete(editor, body) {
  if (!editor) return;
  let api;
  try { api = await editor; } catch (e) { return; }
  if (body.layout.v === 1) api.setLayout(body.layout);
  else if (api.isOpen()) api.close();
}
