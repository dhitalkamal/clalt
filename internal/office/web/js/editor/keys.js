// keyboard shortcuts while build mode is open
import { isTyping } from './dom.js';
import { moveRoom, moveFurniture, rotateFurniture, removeRoom, removeFurniture, cloneLayout } from './model.js';
import { furnitureRect, nextRot } from './geometry.js';

const TOOL_KEYS = { 1: 'select', 2: 'room', 3: 'furniture', 4: 'door' };
const ARROWS = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] };

export function deleteSelection(ed) {
  if (ed.selRoom()) ed.edit((l) => removeRoom(l, ed.sel.id));
  else if (ed.selFurn()) ed.edit((l) => removeFurniture(l, ed.sel.id));
  else return false;
  ed.select(null);
  return true;
}

// r: rotate the selected furniture, or the ghost when nothing is selected
export function rotateCurrent(ed) {
  if (ed.selFurn()) {
    ed.edit((l) => rotateFurniture(l, ed.sel.id, ed.catMap));
    return true;
  }
  if (ed.tool === 'furniture' && ed.furnItem) {
    ed.ghostRot = nextRot(ed.ghostRot);
    return true;
  }
  return false;
}

function nudge(ed, dx, dz) {
  const room = ed.selRoom();
  const f = ed.selFurn();
  const { w, d } = ed.layout.size;
  if (room && room.level === 0) {
    if (room.x + dx < 0 || room.z + dz < 0 || room.x + room.w + dx > w || room.z + room.d + dz > d) return true;
    ed.edit((l) => moveRoom(l, room.id, dx, dz));
  } else if (f && f.level === 0) {
    const r = furnitureRect(f, ed.catMap);
    if (r.x + dx < 0 || r.z + dz < 0 || r.x + r.w + dx > w || r.z + r.d + dz > d) return true;
    ed.edit((l) => moveFurniture(l, ed.catMap, f.id, f.x + dx, f.z + dz));
  } else return false;
  return true;
}

function escape(ed) {
  // an open menu takes the first Esc
  const menu = ed.menus.find((m) => m.isOpen());
  if (menu) {
    menu.close();
    return true;
  }
  if (ed.drag) {
    if (ed.drag.base) ed.layout = cloneLayout(ed.drag.base);
    ed.drag = null;
    ed.dragRect = null;
    ed.changed('drag');
    return true;
  }
  if (ed.tool !== 'select') {
    ed.setTool('select');
    return true;
  }
  if (ed.sel) {
    ed.select(null);
    return true;
  }
  return false;
}

export function attachKeys(ed) {
  const onKey = (ev) => {
    if (!ed.open) return;
    const mod = ev.metaKey || ev.ctrlKey;
    const key = ev.key.length === 1 ? ev.key.toLowerCase() : ev.key;
    const typing = isTyping(ev.target);
    let used = false;
    if (key === 'Escape') used = escape(ed);
    else if (mod && key === 'z' && !typing) {
      ev.shiftKey ? ed.redo() : ed.undo();
      used = true;
    } else if (mod && key === 'y' && !typing) {
      ed.redo();
      used = true;
    } else if (!mod && !typing && !ev.altKey) {
      if (key === 'r') used = rotateCurrent(ed);
      else if (key === 'x' || key === 'Delete' || key === 'Backspace') used = deleteSelection(ed);
      else if (TOOL_KEYS[key]) {
        ed.setTool(TOOL_KEYS[key]);
        used = true;
      } else if (ARROWS[key]) used = nudge(ed, ...ARROWS[key]);
    }
    if (used) {
      ev.preventDefault();
      ev.stopImmediatePropagation();
    }
  };
  window.addEventListener('keydown', onKey, true);
  return () => window.removeEventListener('keydown', onKey, true);
}
