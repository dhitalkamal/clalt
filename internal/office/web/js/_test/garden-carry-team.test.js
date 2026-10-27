// carry decisions for team members and interns across a campus rebuild. pure: plain slot and agent objects.
import test from 'node:test';
import assert from 'node:assert/strict';
import { carryAction, slotMoved } from '../garden/carry.js';

const L1 = { id: 'old' };
const L2 = { id: 'new' };
const slot = (over) => ({ itemId: 'desk-a', kind: 'desk', side: 0, seatKind: 'desk', x: 1, z: 2, level: 0, ...over });
const agent = (s, over = {}) => ({ L: L1, slot: s, mode: 'seated', s: { state: 'busy' }, ...over });
const member = { id: 'team:rev', role: 'team' };
const intern = { id: 'sub1', intern: true };

test('team member whose desk keeps its id is kept and re-pointed', () => {
  const a = agent(slot());
  assert.equal(carryAction(a, slot(), member, L2), 'remap');
  assert.equal(carryAction({ ...a, L: L2 }, slot(), member, L2), 'keep');
});

test('team member at a moved desk walks', () => {
  assert.equal(carryAction(agent(slot()), slot({ itemId: 'desk-b' }), member, L2), 'walk');
});

test('a seatKind change counts as moved even on the same item', () => {
  const stool = slot({ itemId: 'st1', kind: 'round', seatKind: 'stool' });
  const rev = slot({ itemId: 'st1', kind: 'round', seatKind: 'review' });
  assert.equal(slotMoved(stool, rev), true);
  assert.equal(carryAction(agent(stool), rev, intern, L2), 'walk');
});

test('intern whose stool vanished walks to a new stool or hides', () => {
  const stool = slot({ itemId: 's1', kind: 'round', seatKind: 'stool' });
  assert.equal(carryAction(agent(stool), slot({ itemId: 's2', kind: 'round', seatKind: 'stool' }), intern, L2), 'walk');
  assert.equal(carryAction(agent(stool), undefined, intern, L2), 'hide');
});

test('leaving agents without a slot stay, stale ones are dropped', () => {
  const a = agent(slot(), { s: { leaving: true } });
  assert.equal(carryAction(a, undefined, intern, L1), 'keep');
  assert.equal(carryAction(a, undefined, intern, L2), 'drop');
});

test('agents gone from the roster are dropped, leaving ones remap', () => {
  assert.equal(carryAction(agent(slot()), undefined, null, L2), 'drop');
  const a = agent(slot(), { s: { leaving: true } });
  assert.equal(carryAction(a, slot({ itemId: 'desk-b' }), member, L2), 'remap');
});
