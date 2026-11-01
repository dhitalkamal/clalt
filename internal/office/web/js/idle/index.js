// the page side of idle choices: loads the rules at start so the scene follows them, and makes the settings section.
import { app } from '../app.js';
import { idleStore } from './store.js';
import { createIdleSection } from './section.js';

export function startIdle() {
  idleStore.subscribe((rules) => { app.idleRules = rules; });
  idleStore.load();
  return { section: createIdleSection({ store: idleStore, app }) };
}
