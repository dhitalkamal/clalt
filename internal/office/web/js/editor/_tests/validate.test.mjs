import test from 'node:test';
import assert from 'node:assert/strict';
import { validateLayout, seatCounts } from '../validate.js';
import { catalog, baseLayout } from './fixtures.mjs';

const codes = (l, opts) => validateLayout(l, catalog, opts).issues.map((i) => i.code);
const errs = (l) => validateLayout(l, catalog).errors.map((i) => i.code);

test('a good layout has no errors', () => {
  assert.deepEqual(errs(baseLayout()), []);
});

test('exactly one lobby on level 0', () => {
  const none = baseLayout();
  none.rooms.shift();
  assert.ok(errs(none).includes('lobby-missing'));
  const two = baseLayout();
  two.rooms[1].type = 'lobby';
  assert.ok(errs(two).includes('lobby-many'));
  const up = baseLayout();
  up.rooms[2].type = 'lobby';
  assert.ok(errs(up).includes('lobby-level'));
});

test('rooms on the same level must not overlap, other levels may', () => {
  const l = baseLayout();
  l.rooms[1].x = 6;
  const v = validateLayout(l, catalog);
  assert.ok(v.errors.some((i) => i.code === 'room-overlap' && i.id === 'r2'));
  const l2 = baseLayout();
  l2.rooms[2].level = 1;
  assert.equal(errs(l2).includes('room-overlap'), false);
});

test('rooms stay inside the floor, level 1 rooms inside the deck', () => {
  const l = baseLayout();
  l.rooms[1].x = 36;
  assert.ok(errs(l).includes('room-outside'));
  const l2 = baseLayout();
  l2.rooms[2].w = 12;
  assert.ok(errs(l2).includes('room-outside'));
  const l3 = baseLayout();
  l3.deck = null;
  l3.stairs = null;
  assert.ok(errs(l3).includes('room-outside'));
});

test('furniture footprints must not overlap, rotation counts', () => {
  const l = baseLayout();
  l.furniture[1].x = 13;
  l.furniture[1].z = 1;
  assert.ok(errs(l).includes('furn-overlap'));
  const l2 = baseLayout();
  assert.equal(errs(l2).includes('furn-overlap'), false);
  l2.furniture[0].rot = 90;
  assert.ok(errs(l2).includes('furn-overlap'));
});

test('furniture must be inside the floor or deck and known to the catalog', () => {
  const l = baseLayout();
  l.furniture[0].x = 39;
  assert.ok(errs(l).includes('furn-outside'));
  const l2 = baseLayout();
  l2.furniture[0].item = 'ghost';
  assert.ok(errs(l2).includes('furn-unknown'));
  const l3 = baseLayout();
  l3.furniture[0].item = 'sofa';
  l3.furniture[0].level = 1;
  l3.furniture[0].x = 21;
  l3.furniture[0].z = 1;
  assert.ok(errs(l3).includes('furn-level'));
});

test('stairs are required with a deck and must fit the floor', () => {
  const l = baseLayout();
  l.stairs = null;
  assert.ok(errs(l).includes('stairs-missing'));
  const l2 = baseLayout();
  l2.stairs = { x: 99, z: 1, dir: 'n' };
  assert.ok(errs(l2).includes('stairs-outside'));
});

test('ids are well formed and unique, counts are capped', () => {
  const l = baseLayout();
  l.rooms[1].id = 'r1';
  assert.ok(errs(l).includes('id-dup'));
  const l2 = baseLayout();
  l2.furniture[0].id = 'Bad Id';
  assert.ok(errs(l2).includes('id-bad'));
  const l3 = baseLayout();
  for (let i = 0; i < 70; i++) l3.rooms.push({ id: 'x' + i, type: 'play', level: 0, x: 0, z: 0, w: 2, d: 2, label: 'p' });
  assert.ok(errs(l3).includes('rooms-max'));
});

test('colors, size, door and account follow the contract', () => {
  const l = baseLayout();
  l.floorColor = 'red';
  l.walls.color = '#abc';
  assert.equal(errs(l).filter((c) => c === 'color-bad').length, 2);
  const l2 = baseLayout();
  l2.size.w = 4;
  assert.ok(errs(l2).includes('size-range'));
  const l3 = baseLayout();
  l3.door.at = 60;
  assert.ok(errs(l3).includes('door-range'));
  const l4 = baseLayout();
  l4.rooms[0].account = 'work';
  assert.ok(errs(l4).includes('account-type'));
  const l5 = baseLayout();
  l5.rooms[1].floor = 'blue';
  assert.ok(errs(l5).includes('color-bad'));
});

test('issues name the offending room or furniture for red highlights', () => {
  const l = baseLayout();
  l.rooms[1].x = 6;
  const v = validateLayout(l, catalog);
  assert.ok(v.bad.has('r2'));
  assert.ok(v.issues.every((i) => typeof i.msg === 'string' && i.msg.length > 5));
});

test('seatCounts counts seat-tagged furniture inside each room', () => {
  const l = baseLayout();
  const c = seatCounts(l, catalog);
  assert.equal(c.get('r2'), 1);
  assert.equal(c.get('r1'), 0);
});

test('a cabins room with fewer seats than sessions is a warning', () => {
  const l = baseLayout();
  const v = validateLayout(l, catalog, { sessionCount: () => 3 });
  assert.ok(v.warnings.some((i) => i.code === 'seats-short' && i.id === 'r2'));
  assert.deepEqual(v.errors, []);
  const ok = validateLayout(l, catalog, { sessionCount: () => 1 });
  assert.equal(ok.warnings.some((i) => i.code === 'seats-short'), false);
});

test('codes helper stays in sync', () => {
  assert.ok(Array.isArray(codes(baseLayout())));
});

test('an empty or blank title is an error', () => {
  for (const title of ['', '   ', '\t']) {
    const l = baseLayout();
    l.title = title;
    const e = validateLayout(l, catalog, {}).errors.find((i) => i.code === 'title-empty');
    assert.ok(e, JSON.stringify(title));
    assert.equal(e.msg, 'Title is empty.');
  }
  const ok = baseLayout();
  ok.title = 'Mine';
  assert.ok(!errs(ok).includes('title-empty'));
});
