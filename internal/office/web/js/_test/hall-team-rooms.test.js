// team members and their interns sit in the room of their type (review, meeting) when the layout has one
import test from 'node:test';
import assert from 'node:assert/strict';
import { planSeats } from '../routing.js';

const CATALOG = [{ id: 'desk', footprint: { w: 2, d: 1 }, tags: ['seat'] }];
let fid = 0;
const room = (id, type, x, z) => ({ id, type, level: 0, x, z, w: 12, d: 6, label: id });
const desksIn = (r, n) => Array.from({ length: n }, (_, i) => ({ id: 'f' + (fid++), item: 'desk', level: 0, x: r.x + 1 + i * 3, z: r.z + 1, rot: 0, room: r.id }));

function hall(withReview = true) {
  const rooms = [room('lobby', 'lobby', 0, 0), room('ws', 'workspace', 0, 8), room('re', 'research', 0, 16)];
  if (withReview) rooms.push(room('rv', 'review', 14, 0), room('mt', 'meeting', 14, 8));
  const furniture = rooms.filter((r) => ['workspace', 'research', 'review', 'meeting'].includes(r.type)).flatMap((r) => desksIn(r, 3));
  return { v: 1, name: 't', grid: 0.5, size: { w: 40, d: 30 }, door: { side: 'left', at: 2 }, deck: null, stairs: null, rooms, furniture };
}
const member = (team, state = 'busy') => ({ key: 'team:' + team, kind: 'team', team, type: team, state });
const intern = (team, n = 1) => ({ key: 'run:' + team + n, kind: 'intern', team, type: team, state: 'busy' });
const roomOf = (plan, key) => plan.get(key)?.room;

test('a reviewer member and its intern sit in the review room', () => {
  const plan = planSeats(hall(), [member('pr-reviewer'), intern('pr-reviewer')], CATALOG);
  assert.equal(roomOf(plan, 'team:pr-reviewer'), 'rv');
  assert.equal(roomOf(plan, 'run:pr-reviewer1'), 'rv');
});

test('a plan member sits in the meeting room, idle or busy', () => {
  const plan = planSeats(hall(), [member('Plan', 'idle')], CATALOG);
  assert.equal(roomOf(plan, 'team:Plan'), 'mt');
});

test('general-purpose and explore members stay in the work rooms', () => {
  const plan = planSeats(hall(), [member('general-purpose'), member('explore'), intern('explore')], CATALOG);
  for (const k of ['team:general-purpose', 'team:explore', 'run:explore1']) assert.ok(['ws', 're'].includes(roomOf(plan, k)), k);
});

test('a layout without a review room keeps reviewers in the work rooms', () => {
  const plan = planSeats(hall(false), [member('pr-reviewer'), intern('pr-reviewer')], CATALOG);
  for (const k of ['team:pr-reviewer', 'run:pr-reviewer1']) assert.ok(['ws', 're'].includes(roomOf(plan, k)), k);
});

test('a full review room sends the next member to the work rooms', () => {
  const roster = ['a-review', 'b-review', 'c-review', 'd-review'].map((t) => member(t));
  const rooms = roster.map((m) => roomOf(planSeats(hall(), roster, CATALOG), m.key));
  assert.equal(rooms.filter((r) => r === 'rv').length, 3);
  assert.equal(rooms.filter((r) => r === 'ws' || r === 're').length, 1);
});
