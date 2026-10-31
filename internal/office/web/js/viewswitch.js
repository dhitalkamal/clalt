// the order of steps when a new layout arrives, with every side effect injected so it runs in node.
import { camAfterSwitch } from './viewstate.js';

export async function runSwitch(deps, state, body) {
  state.layout = body.layout;
  state.edit = body.edit;
  // the editor edits halls only: a garden must not keep it open
  if (body.layout.v !== 1) deps.closeEditor();
  // the old manager walks the old layout, so it goes before the new one is built
  deps.resetNav();
  state.cam = camAfterSwitch(state.cam);
  deps.saveHash();
  try {
    await deps.apply(body.layout);
    deps.notice('');
  } finally {
    // the camera must come back even when the build threw
    deps.keepClear();
    deps.goCam(state.cam, true);
  }
  deps.maybeEditor(body);
}
