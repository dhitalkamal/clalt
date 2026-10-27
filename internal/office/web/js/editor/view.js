// binds the camera and renderer to cell space: cells to pixels and back
import { projectPoint, groundPoint } from './projection.js';

export function createView(opts) {
  const { camera, renderer, cellToWorld, worldToCell } = opts;
  const el = renderer.domElement;

  const size = () => {
    const r = el.getBoundingClientRect();
    return { w: r.width, h: r.height, left: r.left, top: r.top };
  };
  const refresh = () => {
    camera.updateMatrixWorld && camera.updateMatrixWorld();
  };
  return {
    size,
    refresh,
    // pixel position (relative to the canvas) of a cell corner on a level
    project(level, x, z) {
      const s = size();
      return projectPoint(camera, cellToWorld(level, x, z), s.w, s.h);
    },
    // cell under a pointer event on the given level, or null
    cellAt(ev, level) {
      refresh();
      const s = size();
      const y = cellToWorld(level, 0, 0).y;
      const p = groundPoint(camera, ev.clientX - s.left, ev.clientY - s.top, s.w, s.h, y);
      if (!p) return null;
      const c = worldToCell(level, p);
      return { x: Math.floor(c.x), z: Math.floor(c.z) };
    },
  };
}
