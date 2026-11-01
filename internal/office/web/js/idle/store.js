// the one rules store of the page: the settings page, the quick menu and the scene all read it.
// a 404 means an older server without /api/idle: defaults apply and the controls stay hidden.
import { DEFAULT_RULES, normalizeRules, withChoice } from './resolve.js';

const PATH = '/api/idle';

async function failure(res) {
  let text = '';
  try { text = (await res.text()).trim().split('\n')[0]; } catch (e) { /* the status says enough */ }
  return new Error(text || 'http ' + res.status);
}

export function createIdleStore(fetchFn) {
  const call = (path, init) => (fetchFn || globalThis.fetch)(path, { cache: 'no-store', credentials: 'same-origin', ...init });
  let rules = DEFAULT_RULES;
  let available = true;
  const subs = new Set();
  let queue = Promise.resolve();

  const publish = (next) => { rules = next; for (const fn of [...subs]) fn(rules); };

  async function load() {
    try {
      const res = await call(PATH);
      if (res.status === 404) { available = false; rules = DEFAULT_RULES; return; }
      if (!res.ok) throw await failure(res);
      available = true;
      publish(normalizeRules(await res.json()));
    } catch (e) {
      // offline or broken reply: what we had stays, the next open tries again
    }
  }

  // one saved change, computed from the rules as they are when its turn comes
  function set(scope, id, choice) {
    const run = async () => {
      const body = withChoice(rules, scope, id, choice);
      const res = await call(PATH, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Clalt-Edit': '1' }, body: JSON.stringify(body) });
      if (!res.ok) throw await failure(res);
      publish(normalizeRules(await res.json()));
    };
    const turn = queue.then(run, run);
    queue = turn.catch(() => {});
    return turn;
  }

  return {
    rules: () => rules,
    available: () => available,
    load,
    set,
    subscribe(fn) { subs.add(fn); return () => subs.delete(fn); },
  };
}

export const idleStore = createIdleStore();
