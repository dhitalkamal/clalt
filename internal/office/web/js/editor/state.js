// shared editor state, history and the debounced live preview.
import { createHistory } from './history.js';
import { validateLayout, seatCounts } from './validate.js';
import { cloneLayout } from './model.js';

const PREVIEW_MS = 120;

export function createState(opts) {
  const ed = {
    opts, catalog: opts.catalog, catMap: new Map(opts.catalog.map((c) => [c.id, c])),
    layout: cloneLayout(opts.layout), level: 0, tool: 'select', roomType: 'workspace',
    furnItem: null, ghostRot: 0, sel: null, hover: null, dragRect: null, drag: null,
    menus: [], result: null, seats: new Map(), status: { text: '', kind: '' }, open: false, dirty: false,
  };
  const listeners = {};
  let timer = 0;
  ed.history = createHistory(ed.layout);

  ed.on = (name, fn) => (listeners[name] = listeners[name] || []).push(fn);
  ed.emit = (name, arg) => (listeners[name] || []).forEach((fn) => fn(arg));

  ed.validate = () => {
    ed.result = validateLayout(ed.layout, ed.catalog, { sessionCount: opts.sessionCount });
    ed.seats = seatCounts(ed.layout, ed.catalog);
  };

  // previews run one at a time: calls that arrive while one is in flight
  // collapse into a single follow-up with the latest layout
  let running = false;
  let again = false;
  let disposed = false;
  ed.preview = async () => {
    timer = 0;
    if (disposed) return;
    if (running) {
      again = true;
      return;
    }
    running = true;
    try {
      do {
        again = false;
        try {
          await opts.applyLayout(cloneLayout(ed.layout));
        } catch (e) {
          if (!disposed && !again) ed.setStatus('Preview failed: ' + (e && e.message ? e.message : e), 'bad');
        }
      } while (again && !disposed);
    } finally {
      running = false;
    }
  };
  ed.flushPreview = () => {
    if (timer) {
      clearTimeout(timer);
      ed.preview();
    }
  };

  // layout content changed (live drag or committed edit)
  ed.changed = (source) => {
    ed.validate();
    if (disposed) return;
    clearTimeout(timer);
    timer = setTimeout(ed.preview, PREVIEW_MS);
    ed.emit('change', source);
  };
  ed.live = (fn, source) => {
    fn(ed.layout);
    ed.changed(source);
  };
  // record the current layout as one undo step
  ed.commit = (source) => {
    if (ed.history.push(ed.layout)) ed.dirty = true;
    ed.changed(source);
  };
  ed.edit = (fn, source) => {
    fn(ed.layout);
    ed.commit(source);
  };

  const restore = (next) => {
    if (!next) return;
    ed.drag = null;
    ed.dragRect = null;
    ed.layout = next;
    ed.dirty = true;
    if (ed.sel && !(ed.sel.kind === 'room' ? ed.selRoom() : ed.selFurn())) ed.sel = null;
    ed.changed('history');
    ed.emit('history');
    ed.emit('select');
  };
  ed.undo = () => restore(ed.history.undo());
  ed.redo = () => restore(ed.history.redo());
  // load another layout (template or saved one) as an undoable step
  ed.replaceLayout = (layout) => {
    ed.layout = cloneLayout(layout);
    ed.sel = null;
    ed.history.push(ed.layout);
    ed.dirty = true;
    ed.changed('history');
    ed.emit('history');
    ed.emit('select');
  };
  // fresh baseline, no undo past it (after save or a new session)
  ed.reset = (layout) => {
    ed.layout = cloneLayout(layout);
    ed.history.reset(ed.layout);
    ed.sel = null;
    ed.dirty = false;
    ed.changed('history');
    ed.emit('history');
    ed.emit('select');
  };

  ed.selRoom = () => (ed.sel && ed.sel.kind === 'room' ? ed.layout.rooms.find((r) => r.id === ed.sel.id) : null);
  ed.selFurn = () => (ed.sel && ed.sel.kind === 'furn' ? ed.layout.furniture.find((f) => f.id === ed.sel.id) : null);
  ed.select = (sel) => {
    ed.sel = sel;
    ed.emit('select');
  };
  ed.setTool = (tool) => {
    ed.tool = tool;
    ed.drag = null;
    ed.dragRect = null;
    ed.emit('tool');
  };
  ed.setStatus = (text, kind = '') => {
    ed.status = { text, kind };
    ed.emit('status');
  };
  ed.dispose = () => {
    disposed = true;
    clearTimeout(timer);
  };

  ed.validate();
  return ed;
}
