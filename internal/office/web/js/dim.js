// agents fade while the editor is open so the floor plan reads clearly. character materials are shared between
// agents, so this is one switch for all of them.
import { app } from './app.js';

export function syncDim() {
  if (app.dimmed === app.editing) return;
  app.dimmed = app.editing;
  for (const a of app.agents.values()) {
    a.model.traverse((o) => {
      if (!o.isMesh) return;
      for (const m of [].concat(o.material)) {
        m.transparent = app.editing;
        m.opacity = app.editing ? 0.45 : 1;
        m.depthWrite = !app.editing;
        m.needsUpdate = true;
      }
    });
  }
}
