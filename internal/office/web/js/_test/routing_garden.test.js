// routing tests for layout v2 (garden). seats come from a fake source, no three.js involved.
import test from 'node:test';
import assert from 'node:assert/strict';
import { planSeats, breakSpots } from '../routing.js';

const seat = (pavilionId, type, kind, x, z, extra = {}) => ({ pavilionId, type, account: '', y: 0, x, z, rot: 0, kind, ...extra });

// a small campus: lobby, one cabin per account, research, workspace, treehouse, cafe, play
function campus({ cabinDesks = 2, wsDesks = 2, resDesks = 2, extra = [] } = {}) {
  const seats = [];
  for (let i = 0; i < cabinDesks; i++) seats.push(seat('cab-a', 'cabin', 'desk', 0 + i * 2, 0, { account: 'a' }));
  for (let i = 0; i < cabinDesks; i++) seats.push(seat('cab-b', 'cabin', 'desk', 0 + i * 2, 8, { account: 'b' }));
  for (let i = 0; i < resDesks; i++) seats.push(seat('res', 'research', 'desk', 20 + i * 2, 0));
  for (let i = 0; i < wsDesks; i++) seats.push(seat('ws', 'workspace', 'desk', 20 + i * 2, 10));
  seats.push(seat('th', 'treehouse', 'review', 40, 0, { y: 2 }), seat('th', 'treehouse', 'meeting', 44, 0, { y: 2 }));
  seats.push(seat('cafe', 'cafe', 'cafe', 30, 20), seat('cafe', 'cafe', 'cafe', 31, 20));
  seats.push(seat('play', 'play', 'play', 36, 20));
  seats.push(seat('lob', 'lobby', 'lobby', -5, 0), seat('lob', 'lobby', 'lobby', -5, 1), seat('lob', 'lobby', 'lobby', -5, 2));
  const overflow = [
    { x: 19, z: 12, rot: 0, near: 'workspace' }, { x: 21, z: 12, rot: 0, near: 'workspace' },
    { x: 30, z: 23, rot: 0, near: 'cafe' }, { x: -4, z: 4, rot: 0, near: 'lobby' },
  ];
  return {
    v: 2, name: 't',
    pavilions: [
      { id: 'lob', type: 'lobby', x: -5, z: 0 }, { id: 'cab-a', type: 'cabin', x: 0, z: 0, account: 'a' },
      { id: 'cab-b', type: 'cabin', x: 0, z: 8, account: 'b' }, { id: 'res', type: 'research', x: 20, z: 0 },
      { id: 'ws', type: 'workspace', x: 20, z: 10 }, { id: 'th', type: 'treehouse', x: 42, z: 0 },
      { id: 'cafe', type: 'cafe', x: 30, z: 20 }, { id: 'play', type: 'play', x: 36, z: 20 },
    ],
    overflow,
    gardenSeats: seats.concat(extra),
  };
}
const garden = (l) => ({ seats: l.gardenSeats, overflow: l.overflow });
const plan = (l, roster, prev) => planSeats(l, roster, null, prev, garden(l));
const main = (key, account = 'a', state = 'busy') => ({ key, kind: 'main', account, type: 'main', state });
const sub = (key, type, account = 'a') => ({ key, kind: 'sub', account, type, state: 'busy' });

test('v2 routes each agent to its room kind', () => {
  const l = campus();
  const p = plan(l, [main('m1', 'a'), main('m2', 'b'), sub('s1', 'Explore'), sub('s2', 'code-reviewer'), sub('s3', 'Plan'), sub('s4', 'general-purpose'), sub('s5', 'mystery')]);
  assert.equal(p.get('m1').room, 'cab-a');
  assert.equal(p.get('m2').room, 'cab-b');
  assert.equal(p.get('s1').room, 'res');
  assert.equal(p.get('s2').room, 'th');
  assert.equal(p.get('s3').room, 'th');
  assert.equal(p.get('s4').room, 'ws');
  assert.equal(p.get('s5').room, 'ws');
  for (const v of p.values()) assert.equal(v.kind, 'seat');
});

test('reviewers use the review side and Plan the meeting side of the treehouse', () => {
  const l = campus();
  const p = plan(l, [sub('r', 'test-runner'), sub('p', 'Plan')]);
  assert.equal(p.get('r').x, 40);
  assert.equal(p.get('p').x, 44);
  assert.equal(p.get('r').level, 2);
});

test('a seat entry carries world coordinates and rotation', () => {
  const l = campus();
  const e = plan(l, [sub('s', 'Explore')]).get('s');
  assert.deepEqual([e.x, e.z, e.rot, e.level], [20, 0, 0, 0]);
  assert.equal(e.world, true);
});

test('a full room spills to overflow nearest the wanted room, then the lobby', () => {
  const l = campus({ wsDesks: 1, resDesks: 0 });
  const roster = ['a', 'b', 'c', 'd', 'e'].map((k) => sub(k, 'general-purpose'));
  const p = plan(l, roster);
  assert.equal(p.get('a').kind, 'seat');
  // the two workspace overflow spots are nearest the workspace
  assert.deepEqual([p.get('b').x, p.get('b').z], [19, 12]);
  assert.deepEqual([p.get('c').x, p.get('c').z], [21, 12]);
  assert.equal(p.get('b').kind, 'stand');
  // then the cafe spot is nearer than the lobby one
  assert.deepEqual([p.get('d').x, p.get('d').z], [30, 23]);
  assert.deepEqual([p.get('e').x, p.get('e').z], [-4, 4]);
});

test('with no overflow left the lobby seats take the rest and then reuse', () => {
  const l = campus({ wsDesks: 1, resDesks: 0 });
  l.overflow = [];
  const roster = ['a', 'b', 'c', 'd', 'e', 'f'].map((k) => sub(k, 'general-purpose'));
  const p = plan(l, roster);
  const at = (k) => [p.get(k).x, p.get(k).z].join(',');
  assert.equal(p.get('a').room, 'ws');
  assert.deepEqual(['b', 'c', 'd'].map(at), ['-5,0', '-5,1', '-5,2']);
  assert.equal(p.get('e').kind, 'stand');
  assert.equal(p.get('f').kind, 'stand');
});

test('a main agent with no cabin of its account stands in the lobby area, never in another cabin', () => {
  const l = campus();
  const p = plan(l, [main('m', 'zzz')]);
  assert.notEqual(p.get('m').room, 'cab-a');
  assert.notEqual(p.get('m').room, 'cab-b');
});

test('arriving agents go to the lobby first', () => {
  const l = campus();
  const p = plan(l, [{ ...main('m', 'a'), state: 'arriving' }]);
  assert.equal(p.get('m').room, 'lob');
});

test('agents keep their seats when the layout grows', () => {
  const l = campus();
  const roster = [main('m1', 'a'), sub('s1', 'general-purpose'), sub('s2', 'general-purpose')];
  const p1 = plan(l, roster);
  // a new workspace pavilion with a seat listed first, new cabin desks for a
  const grown = campus({ cabinDesks: 4, extra: [] });
  grown.gardenSeats.unshift(seat('ws2', 'workspace', 'desk', 50, 50), seat('cab-a', 'cabin', 'desk', -2, 0, { account: 'a' }));
  const p2 = plan(grown, roster, p1);
  for (const k of ['m1', 's1', 's2']) assert.deepEqual([p2.get(k).x, p2.get(k).z, p2.get(k).seat], [p1.get(k).x, p1.get(k).z, p1.get(k).seat]);
});

test('an agent whose seat vanished gets a new one', () => {
  const l = campus();
  const p1 = plan(l, [sub('s1', 'Explore')]);
  const shrunk = campus();
  shrunk.gardenSeats = shrunk.gardenSeats.filter((s) => !(s.type === 'research' && s.x === 20));
  const p2 = plan(shrunk, [sub('s1', 'Explore')], p1);
  assert.equal(p2.get('s1').x, 22);
});

test('the result does not depend on roster order', () => {
  const l = campus();
  const r = [main('b', 'a'), main('a', 'a'), sub('z', 'Explore'), sub('y', 'Explore')];
  const one = plan(l, r);
  const two = plan(l, [...r].reverse());
  for (const k of ['a', 'b', 'y', 'z']) assert.equal(one.get(k).seat, two.get(k).seat);
});

test('v2 break spots are the cafe and play seats, lobby when there are none', () => {
  const l = campus();
  const b = breakSpots(l, null, garden(l));
  assert.equal(b.cafe.length, 2);
  assert.equal(b.play.length, 1);
  assert.equal(b.all.length, 3);
  assert.equal(b.cafe[0].kind, 'cafe');
  const bare = campus();
  bare.gardenSeats = bare.gardenSeats.filter((s) => s.kind !== 'cafe' && s.kind !== 'play');
  const nb = breakSpots(bare, null, garden(bare));
  assert.equal(nb.all.length, 3);
  assert.equal(nb.all[0].kind, 'lobby');
});

test('without a garden source v2 still plans: everyone stands near the lobby pavilion', () => {
  const l = campus();
  const p = planSeats(l, [main('m')]);
  assert.equal(p.get('m').kind, 'stand');
});
