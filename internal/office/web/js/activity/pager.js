// paging and refresh state for one open log. pure: every function returns a new state.
// entries are newest first. next is the cursor for older entries, '' when there are none.
import { mergeNewer, mergeOlder } from './merge.js';

export const activityPath = ({ handle, agent, before, after, limit }) => {
  const q = new URLSearchParams({ handle });
  if (agent) q.set('agent', agent);
  if (before) q.set('before', before);
  if (after) q.set('after', after);
  if (limit) q.set('limit', String(limit));
  return '/api/activity?' + q.toString();
};

export const pagerKey = (t) => t.handle + '/' + (t.agent || '');

export const newPager = (target) => ({
  target: { handle: target.handle, agent: target.agent || '' },
  entries: [], next: '', newest: '', loaded: false, refreshing: false, loadingOlder: false, error: '', olderError: '',
});

// true when a reply is for the run this pager shows
export const matches = (p, reply) => reply.handle === p.target.handle && (reply.agent || '') === p.target.agent;

const list = (reply) => (Array.isArray(reply.entries) ? reply.entries : []);

export const applyFirst = (p, reply) => ({ ...p, entries: list(reply), next: reply.next || '', newest: reply.newest || '', loaded: true, error: '' });

// what a refresh tick found (see catchup.js): new entries go in front, the older cursor stays.
// newest moves forward only when the route named one.
export function applyNewer(p, batch) {
  const m = mergeNewer(p.entries, batch.entries || []);
  return { pager: { ...p, entries: m.entries, newest: batch.newest || p.newest, error: '' }, added: m.added };
}

export const startOlder = (p) => ({ ...p, loadingOlder: true, olderError: '' });
export const failOlder = (p, msg) => ({ ...p, loadingOlder: false, olderError: msg });
export const applyOlder = (p, reply) => ({ ...p, entries: mergeOlder(p.entries, list(reply)).entries, next: reply.next || '', loadingOlder: false, olderError: '' });

export const canLoadOlder = (p) => p.loaded && p.next !== '' && !p.loadingOlder;

export const shouldRefresh = ({ open, hidden, loaded, refreshing }) => open && !hidden && loaded && !refreshing;
