// the page's one activity panel, made on first use. input.js calls openActivity with a roster record.
import { app, ui, canvas } from '../app.js';
import { createActivityPanel } from './panel.js';

let panel = null;

export function openActivity(rec) {
  if (!panel) {
    // the canvas takes focus back when the character that was clicked is not a focusable opener
    canvas.tabIndex = -1;
    panel = createActivityPanel({ ctx: () => ({ accounts: ui.accounts, sessions: app.sessions }), fallback: canvas });
  }
  panel.open(rec);
}
