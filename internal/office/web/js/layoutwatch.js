// keeps the page on the right layout: select() loads a template and always applies it, check() is called
// from the snapshot poll and applies again only when the server's rev for that template changed.
import { loadLayout } from './layoutload.js';

const EVERY = 3000;

export function createLayoutWatcher({ load = loadLayout, apply, every = EVERY, now = () => Date.now() }) {
  let template = null;
  let rev = null;
  let last = 0;
  let busy = false;
  let want = 0;
  let queue = Promise.resolve();

  // every apply runs on one chain so two scenes never build at once. a job that is no longer the latest
  // request is dropped, before it loads and again before it applies.
  function enqueue(id, force) {
    const mine = ++want;
    const run = queue.then(async () => {
      if (mine !== want) return null;
      const body = await load(id);
      if (mine !== want) return null;
      if (!force && body.rev === rev) return body;
      const prev = { template, rev };
      template = id;
      rev = body.rev;
      try { await apply(body); } catch (e) { ({ template, rev } = prev); throw e; }
      return body;
    });
    queue = run.catch(() => {});
    return run;
  }

  return {
    get template() { return template; },
    // the page loaded this template and rev itself: check() compares against them from now on
    adopt(id, r) { template = id; rev = r; last = now(); },
    async select(id) {
      last = now();
      return enqueue(id, true);
    },
    async check() {
      if (busy || now() - last < every) return;
      busy = true;
      last = now();
      try { await enqueue(template, false); } catch (e) { /* the snapshot poll reports lost connections */ } finally { busy = false; }
    },
  };
}
