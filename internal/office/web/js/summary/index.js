// the Today button of the controls row opens the summary panel, made on first use.
import { canvas } from '../app.js';
import { createSummaryPanel } from './panel.js';

let panel = null;

export function initSummary() {
  const b = document.getElementById('b-today');
  if (!b) return;
  b.addEventListener('click', () => {
    if (!panel) {
      canvas.tabIndex = -1;
      panel = createSummaryPanel({ fallback: b });
    }
    panel.toggle();
  });
}
