// pointer tools on the 3d canvas: select, room, furniture and door.
// level 0 only: upper level items are drawn but never hit.
import { rectFromPoints, resizeRect, MIN_ROOM, clampRectInto, furnitureRect, nearestDoor } from './geometry.js';
import { hitFurniture, hitRoom, placeAnchor, canPlace } from './picking.js';
import { addRoom, addFurniture, moveRoom, moveFurniture, cloneLayout, findRoom, findFurniture } from './model.js';
import { handleScreen } from './overlay.js';

const HANDLE_PX = 10;

export function attachPointer(ed, view, el) {
  const floor = () => ({ x: 0, z: 0, w: ed.layout.size.w, d: ed.layout.size.d });
  const inFloor = (c) => c && c.x >= 0 && c.z >= 0 && c.x < ed.layout.size.w && c.z < ed.layout.size.d;

  const handleAt = (ev) => {
    const s = view.size();
    const px = ev.clientX - s.left;
    const py = ev.clientY - s.top;
    const hit = handleScreen(ed, view).find((p) => Math.abs(p.x - px) <= HANDLE_PX && Math.abs(p.y - py) <= HANDLE_PX);
    return hit ? hit.h : null;
  };

  function begin(ev, cell) {
    const room = ed.selRoom();
    const handle = room && handleAt(ev);
    if (handle) {
      ed.drag = { type: 'resize', start: cell, base: cloneLayout(ed.layout), id: room.id, handle };
      return;
    }
    const f = hitFurniture(ed.layout, ed.catMap, 0, cell.x, cell.z);
    if (f) {
      ed.select({ kind: 'furn', id: f.id });
      ed.drag = { type: 'move-furn', start: cell, base: cloneLayout(ed.layout), id: f.id, off: { x: cell.x - f.x, z: cell.z - f.z } };
      return;
    }
    if (ed.tool === 'furniture' && ed.furnItem) return place(cell);
    const r = hitRoom(ed.layout, 0, cell.x, cell.z);
    if (r) {
      ed.select({ kind: 'room', id: r.id });
      ed.drag = { type: 'move-room', start: cell, base: cloneLayout(ed.layout), id: r.id };
      return;
    }
    ed.select(null);
    if (ed.tool === 'room') {
      ed.drag = { type: 'create', start: cell };
      ed.dragRect = rectFromPoints(cell.x, cell.z, cell.x, cell.z);
    }
  }

  function place(cell) {
    const a = placeAnchor(ed.catMap, ed.furnItem, ed.ghostRot, cell.x, cell.z);
    if (!a || !canPlace(ed.layout, ed.catMap, ed.furnItem, ed.ghostRot, 0, a.x, a.z)) {
      ed.setStatus('Cannot place there: it overlaps something or leaves the floor.', 'bad');
      return;
    }
    ed.edit((l) => addFurniture(l, ed.catMap, ed.furnItem, 0, a.x, a.z, ed.ghostRot));
    // stay in placing mode: nothing selected so r keeps rotating the ghost
    ed.select(null);
  }

  function door(cell) {
    ed.edit((l) => (l.door = nearestDoor(l, cell.x, cell.z)));
  }

  function drag(cell) {
    const d = ed.drag;
    const dx = cell.x - d.start.x;
    const dz = cell.z - d.start.z;
    if (d.type === 'create') {
      const r = rectFromPoints(d.start.x, d.start.z, cell.x, cell.z);
      ed.dragRect = clampRectInto(r, floor());
      return;
    }
    const next = cloneLayout(d.base);
    if (d.type === 'move-room') {
      const room = findRoom(next, d.id);
      const lim = floor();
      const cx = Math.max(lim.x - room.x, Math.min(dx, lim.w - room.w - room.x));
      const cz = Math.max(lim.z - room.z, Math.min(dz, lim.d - room.d - room.z));
      moveRoom(next, d.id, cx, cz);
    } else if (d.type === 'resize') {
      const room = findRoom(next, d.id);
      Object.assign(room, resizeRect(room, d.handle, dx, dz, MIN_ROOM));
    } else if (d.type === 'move-furn') {
      const f = findFurniture(next, d.id);
      const r = furnitureRect(f, ed.catMap);
      const moved = clampRectInto({ ...r, x: cell.x - d.off.x, z: cell.z - d.off.z }, floor());
      moveFurniture(next, ed.catMap, d.id, moved.x, moved.z);
    }
    ed.layout = next;
    ed.changed('drag');
  }

  function finish() {
    const d = ed.drag;
    ed.drag = null;
    if (!d) return;
    if (d.type === 'create') {
      const r = ed.dragRect;
      ed.dragRect = null;
      if (r.w < MIN_ROOM || r.d < MIN_ROOM) {
        ed.setStatus('Drag a bigger rectangle: rooms are at least ' + MIN_ROOM + ' by ' + MIN_ROOM + ' cells.');
        return;
      }
      let made;
      ed.edit((l) => (made = addRoom(l, r, ed.roomType, 0)));
      ed.select({ kind: 'room', id: made.id });
      return;
    }
    ed.commit('drag');
  }

  const down = (ev) => {
    if (!ed.open || ev.button !== 0 || ev.target !== el) return;
    ev.preventDefault();
    ev.stopImmediatePropagation();
    const cell = view.cellAt(ev, 0);
    if (!inFloor(cell)) {
      ed.select(null);
      return;
    }
    if (ed.tool === 'door') door(cell);
    else begin(ev, cell);
  };
  const move = (ev) => {
    if (!ed.open) return;
    const cell = view.cellAt(ev, 0);
    ed.hover = inFloor(cell) && (ed.drag || ev.target === el) ? cell : null;
    if (ed.drag && cell) drag(cell);
  };
  const up = () => {
    if (ed.open && ed.drag) finish();
  };
  const cancel = () => {
    ed.drag = null;
    ed.dragRect = null;
  };

  window.addEventListener('pointerdown', down, true);
  window.addEventListener('pointermove', move, true);
  window.addEventListener('pointerup', up, true);
  window.addEventListener('pointercancel', cancel, true);
  return () => {
    window.removeEventListener('pointerdown', down, true);
    window.removeEventListener('pointermove', move, true);
    window.removeEventListener('pointerup', up, true);
    window.removeEventListener('pointercancel', cancel, true);
  };
}
