// the page's one activity panel, made on first use. input.js calls openActivity with a roster record.
import { app, ui, canvas } from '../app.js';
import { createActivityPanel } from './panel.js';
import { releaseSelection } from './release.js';

let panel = null;

export function openActivity(rec) {
  if (!panel) {
    // the canvas takes focus back when the character that was clicked is not a focusable opener
    canvas.tabIndex = -1;
    panel = createActivityPanel({ ctx: () => ({ accounts: ui.accounts, sessions: app.sessions }), fallback: canvas, onClose: () => releaseSelection(app, ui) });
  }
  panel.open(rec);
}
