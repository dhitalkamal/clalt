// the one rules store of the page: the settings page, the quick menu and the scene all read it.
// a 404 means an older server without /api/idle: defaults apply and the controls stay hidden.
import { DEFAULT_RULES, normalizeRules, withChoice } from './resolve.js';
import { failText } from '../lostaccess.js';

const PATH = '/api/idle';

async function failure(res) {
  let text = '';
  try { text = (await res.text()).trim().split('\n')[0]; } catch (e) { /* the status says enough */ }
  return new Error(failText(res.status, text));
}

// a save that finds the file changed is retried on the server's rules this many times
const TRIES = 3;
const tagOf = (res) => (res.headers && res.headers.get && res.headers.get('ETag')) || null;

export function createIdleStore(fetchFn) {
  const call = (path, init) => (fetchFn || globalThis.fetch)(path, { cache: 'no-store', credentials: 'same-origin', ...init });
  let rules = DEFAULT_RULES;
  let version = null;
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
      version = tagOf(res);
      publish(normalizeRules(await res.json()));
    } catch (e) {
      // offline or broken reply: what we had stays, the next open tries again
    }
  }

  // one saved change, computed from the rules as they are when its turn comes. on a 409 the
  // server's rules are adopted and only this change is applied again on top of them
  function set(scope, id, choice) {
    const run = async () => {
      for (let i = 0; i < TRIES; i++) {
        const body = withChoice(rules, scope, id, choice);
        const headers = { 'Content-Type': 'application/json', 'X-Clalt-Edit': '1' };
        if (version) headers['If-Match'] = version;
        const res = await call(PATH, { method: 'POST', headers, body: JSON.stringify(body) });
        if (res.status === 409) {
          const theirs = await res.json();
          version = tagOf(res) || (theirs.version ? '"' + theirs.version + '"' : null);
          publish(normalizeRules(theirs.rules));
          continue;
        }
        if (!res.ok) throw await failure(res);
        version = tagOf(res);
        publish(normalizeRules(await res.json()));
        return;
      }
      throw new Error('The idle choices changed elsewhere again and again. Try once more.');
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
