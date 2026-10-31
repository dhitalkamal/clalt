// office editor entry. build mode on top of the 3d page: works on a copy of
// the layout, previews through applyLayout, saves through save(layout).
import { h } from './dom.js';
import { createState } from './state.js';
import { createView } from './view.js';
import { drawOverlay } from './overlay.js';
import { attachPointer } from './tools.js';
import { attachKeys } from './keys.js';
import { createToolbar } from './toolbar.js';
import { createInspector } from './inspector.js';
import { createCatalogPanel } from './catalogPanel.js';
import { createIssuesPanel } from './issuesPanel.js';
import { cloneLayout } from './model.js';

// opts: layout, catalog, applyLayout, cellToWorld, worldToCell, save, camera,
// renderer, scene (unused, accepted), dom (parent, default body), onOpen, onClose,
// templates() -> [{name, title, layout}], layouts {list(), use(name), remove?(name, wasActive)},
// sessionCount (number or room => number), accounts [{id, name}], button (existing element)
export function mountEditor(opts) {
  const ed = createState(opts);
  const view = createView(opts);
  const parent = opts.dom || document.body;
  const el = opts.renderer.domElement;
  const canvas = h('canvas', { class: 'ed-overlay', 'aria-hidden': 'true', hidden: true });
  const button = opts.button || h('button', { type: 'button', class: 'ed-build', text: 'Build' });
  button.setAttribute('aria-pressed', 'false');

  let saving = false;
  async function save() {
    if (saving) return;
    const n = ed.result.errors.length;
    if (n) {
      ed.setStatus('Cannot save yet: ' + n + ' problem' + (n === 1 ? '' : 's') + '. ' + ed.result.errors[0].msg, 'bad');
      return;
    }
    saving = true;
    toolbar.setSaving(true);
    ed.setStatus('Saving...');
    try {
      const res = await opts.save(cloneLayout(ed.layout));
      const err = typeof res === 'string' ? res : res && res.ok === false ? res.error || 'save failed' : '';
      if (err) throw new Error(err);
      ed.dirty = false;
      ed.setStatus('Saved.', 'ok');
    } catch (e) {
      ed.setStatus('Save failed: ' + (e && e.message ? e.message : e), 'bad');
    } finally {
      saving = false;
      toolbar.setSaving(false);
    }
  }

  const toolbar = createToolbar(ed, { save });
  const side = h('aside', { class: 'ed-side', 'aria-label': 'Editor panels' },
    createCatalogPanel(ed).el, createInspector(ed).el, createIssuesPanel(ed).el);
  const root = h('div', { class: 'ed-root' }, toolbar.el, side);
  root.hidden = true;
  parent.append(canvas, root);
  if (!opts.button) parent.append(button);

  const stopPointer = attachPointer(ed, view, el);
  const stopKeys = attachKeys(ed);
  let raf = 0;
  const tick = () => {
    if (!ed.open) return;
    drawOverlay(canvas, ed, view);
    raf = requestAnimationFrame(tick);
  };

  const api = {
    isOpen: () => ed.open,
    open() {
      if (ed.open) return;
      ed.open = true;
      root.hidden = false;
      canvas.hidden = false;
      button.setAttribute('aria-pressed', 'true');
      button.classList.add('on');
      el.classList.add('ed-active');
      if (opts.onOpen) opts.onOpen();
      raf = requestAnimationFrame(tick);
    },
    close() {
      if (!ed.open) return;
      ed.flushPreview();
      ed.open = false;
      ed.drag = null;
      ed.dragRect = null;
      root.hidden = true;
      canvas.hidden = true;
      button.setAttribute('aria-pressed', 'false');
      button.classList.remove('on');
      el.classList.remove('ed-active');
      cancelAnimationFrame(raf);
      if (opts.onClose) opts.onClose();
    },
    // load a layout as the new baseline (after the server changed it)
    setLayout: (layout) => ed.reset(layout),
    getLayout: () => cloneLayout(ed.layout),
    isDirty: () => ed.dirty,
    dispose() {
      api.close();
      stopPointer();
      stopKeys();
      toolbar.dispose();
      ed.dispose();
      button.removeEventListener('click', toggle);
      canvas.remove();
      root.remove();
      if (!opts.button) button.remove();
    },
  };
  const toggle = () => (ed.open ? api.close() : api.open());
  button.addEventListener('click', toggle);
  return api;
}
