// the number on a room label counts the agents working in it, not the ones just sitting there
import test from 'node:test';
import assert from 'node:assert/strict';
import { workingByRoom } from '../roomcount.js';

const slots = new Map([['a', { room: 'r1' }], ['b', { room: 'r1' }], ['c', { room: 'r2' }], ['d', {}]]);
const states = (o) => new Map(Object.entries(o));

test('only busy agents are counted per room', () => {
  const n = workingByRoom(slots, states({ a: 'busy', b: 'idle', c: 'busy', d: 'busy' }));
  assert.equal(n.get('r1'), 1);
  assert.equal(n.get('r2'), 1);
});

test('a room with only idle agents has no entry', () => {
  const n = workingByRoom(slots, states({ a: 'idle', b: 'idle', c: 'needs-you' }));
  assert.equal(n.get('r1') || 0, 0);
  assert.equal(n.get('r2') || 0, 0);
});

test('an agent without a known record is not counted', () => {
  assert.equal(workingByRoom(slots, states({})).size, 0);
});

test('a hidden agent is idle by definition, so it adds nothing to the number', () => {
  // idle choice "hide" only applies while idle: the state map says idle, the count skips it
  const n = workingByRoom(slots, states({ a: 'idle', b: 'busy' }));
  assert.equal(n.get('r1'), 1);
});
