// the cases of the Go validator tests (internal/office/layout/validate_test.go)
// ported as fixtures, so both validators answer the same questions.
import test from 'node:test';
import assert from 'node:assert/strict';
import { validateLayout } from '../validate.js';

const cat = [
  { id: 'desk', footprint: { w: 2, d: 1 }, levels: [0, 1], tags: ['seat'] },
  { id: 'plant', footprint: { w: 1, d: 1 }, levels: [0], tags: [] },
  { id: 'rug', footprint: { w: 4, d: 3 }, levels: [0, 1], tags: ['floor'] },
  { id: 'sofa', footprint: { w: 3, d: 2 }, levels: [0, 1], tags: [] },
];

const valid = () => ({
  v: 1, name: 'mine', title: 'Mine', grid: 0.5, size: { w: 20, d: 12 },
  walls: { color: '#e9e4dc' }, floorColor: '#d9b88f', door: { side: 'front', at: 2 },
  deck: { x: 10, z: 0, w: 10, d: 6 }, stairs: { x: 17, z: 6, dir: 's' },
  rooms: [
    { id: 'r1', type: 'lobby', level: 0, x: 0, z: 0, w: 5, d: 5, label: 'Lobby' },
    { id: 'r2', type: 'workspace', level: 0, x: 6, z: 0, w: 4, d: 5, label: 'Open', floor: '#aabbcc' },
    { id: 'r3', type: 'cabins', level: 1, x: 10, z: 0, w: 5, d: 5, label: 'Cabins', account: 'work' },
  ],
  furniture: [
    { id: 'f1', item: 'desk', level: 0, x: 7, z: 1, rot: 0, room: 'r2' },
    { id: 'f2', item: 'desk', level: 1, x: 11, z: 1, rot: 90 },
  ],
});

const rep = (s, n) => s.repeat(n);
const bad = {
  'version': (l) => (l.v = 2),
  'bad name': (l) => (l.name = '../x'),
  'upper name': (l) => (l.name = 'Mine'),
  'long name': (l) => (l.name = rep('a', 41)),
  'long title': (l) => (l.title = rep('a', 61)),
  'grid': (l) => (l.grid = 1),
  'size small': (l) => (l.size.w = 7),
  'size big': (l) => (l.size.d = 121),
  'wall colour': (l) => (l.walls.color = 'red'),
  'wall colour short': (l) => (l.walls.color = '#fff'),
  'floor colour': (l) => (l.floorColor = 'url(http://x)'),
  'room floor colour': (l) => (l.rooms[1].floor = '#GGGGGG'),
  'door side': (l) => (l.door.side = 'top'),
  'door past front': (l) => (l.door.at = 20),
  'door past left': (l) => (l.door = { side: 'left', at: 12 }),
  'deck outside': (l) => (l.deck = { x: 15, z: 0, w: 10, d: 6 }),
  'deck empty': (l) => (l.deck = { x: 15, z: 0, w: 0, d: 6 }),
  'stairs missing': (l) => (l.stairs = null),
  'stairs without deck': (l) => { l.deck = null; l.rooms = l.rooms.slice(0, 2); l.furniture = l.furniture.slice(0, 1); },
  'stairs dir': (l) => (l.stairs.dir = 'up'),
  'stairs outside': (l) => (l.stairs.x = 20),
  'stairs flight outside': (l) => (l.stairs = { x: 5, z: 3, dir: 's' }),
  'stairs landing outside': (l) => (l.stairs = { x: 5, z: 1, dir: 'n' }),
  'no lobby': (l) => (l.rooms[0].type = 'play'),
  'two lobbies': (l) => (l.rooms[1].type = 'lobby'),
  'lobby on deck': (l) => { l.rooms[0].type = 'play'; l.rooms[2].type = 'lobby'; delete l.rooms[2].account; },
  'room type': (l) => (l.rooms[1].type = 'dungeon'),
  'room id': (l) => (l.rooms[1].id = 'R 2'),
  'dup room id': (l) => (l.rooms[1].id = 'r1'),
  'room level': (l) => (l.rooms[1].level = 2),
  'room outside': (l) => (l.rooms[1].x = 18),
  'room size zero': (l) => (l.rooms[1].w = 0),
  'room negative': (l) => (l.rooms[1].z = -1),
  'room outside deck': (l) => (l.rooms[2].x = 8),
  'rooms overlap': (l) => (l.rooms[1].x = 4),
  'long label': (l) => (l.rooms[1].label = rep('a', 41)),
  'account on workspace': (l) => (l.rooms[1].account = 'work'),
  'account bad': (l) => (l.rooms[2].account = 'a b'),
  'account email': (l) => (l.rooms[2].account = 'me@example.com'),
  'url label': (l) => (l.rooms[1].label = 'see http://evil.example'),
  'url title': (l) => (l.title = '//evil.example/x'),
  'url www': (l) => (l.rooms[1].label = 'www.evil.example'),
  'url javascript': (l) => (l.title = 'javascript:alert(1)'),
  'url data': (l) => (l.title = 'data:text/html,x'),
  'unknown item': (l) => (l.furniture[0].item = 'dragon'),
  'item wrong level': (l) => (l.furniture[1].item = 'plant'),
  'furniture id': (l) => (l.furniture[0].id = ''),
  'dup furniture id': (l) => (l.furniture[1].id = 'f1'),
  'furniture rot': (l) => (l.furniture[0].rot = 45),
  'furniture outside': (l) => (l.furniture[0].x = 19),
  'furniture outside deck': (l) => (l.furniture[1].z = 5),
  'furniture room unknown': (l) => (l.furniture[0].room = 'nope'),
  'furniture room level': (l) => (l.furniture[0].room = 'r3'),
  'furniture overlap': (l) => l.furniture.push({ id: 'f3', item: 'desk', level: 0, x: 8, z: 1, rot: 0 }),
  'too many rooms': (l) => { for (let i = 0; i < 64; i++) l.rooms.push({ id: 'x' + i, type: 'play', level: 0, x: 0, z: 0, w: 1, d: 1, label: 'p' }); },
  'too many furniture': (l) => {
    l.size = { w: 120, d: 120 }; l.furniture = [];
    for (let i = 0; i < 1501; i++) l.furniture.push({ id: 'f' + i, item: 'plant', level: 0, x: i % 100, z: Math.floor(i / 100), rot: 0 });
  },
};

const errs = (l) => validateLayout(l, cat).errors;

test('the shared valid layout passes', () => assert.deepEqual(errs(valid()), []));

for (const [name, mod] of Object.entries(bad)) {
  test('go case refused: ' + name, () => {
    const l = valid();
    mod(l);
    assert.ok(errs(l).length > 0, name + ' passed');
  });
}

test('rotated footprint overlap, and a rotated box leaving the level', () => {
  const l = valid();
  l.furniture = [{ id: 'a', item: 'desk', level: 0, x: 7, z: 1, rot: 90 }, { id: 'b', item: 'plant', level: 0, x: 7, z: 2, rot: 0 }];
  assert.ok(errs(l).some((e) => e.code === 'furn-overlap'));
  l.furniture[0].rot = 0;
  assert.deepEqual(errs(l), []);
  l.furniture = [{ id: 'a', item: 'desk', level: 0, x: 7, z: 11, rot: 90 }, { id: 'b', item: 'plant', level: 0, x: 1, z: 11, rot: 0 }];
  assert.ok(errs(l).some((e) => e.code === 'furn-outside'));
});

test('furniture on different levels may share a cell', () => {
  const l = valid();
  l.furniture = [{ id: 'a', item: 'desk', level: 0, x: 11, z: 1, rot: 0 }, { id: 'b', item: 'desk', level: 1, x: 11, z: 1, rot: 0 }];
  assert.deepEqual(errs(l), []);
});

test('no deck means no upper level', () => {
  const l = valid();
  l.deck = null; l.stairs = null; l.rooms = l.rooms.slice(0, 2); l.furniture = l.furniture.slice(0, 1);
  assert.deepEqual(errs(l), []);
  l.furniture.push({ id: 'up', item: 'desk', level: 1, x: 11, z: 1, rot: 0 });
  assert.ok(errs(l).length > 0);
});

test('stairs in every direction pass', () => {
  for (const st of [{ x: 10, z: 3, dir: 'n' }, { x: 10, z: 6, dir: 's' }, { x: 8, z: 6, dir: 'e' }, { x: 4, z: 6, dir: 'w' }]) {
    const l = valid();
    l.stairs = st;
    assert.deepEqual(errs(l), [], JSON.stringify(st));
  }
});

test('floor items may lie under others and under each other', () => {
  const l = valid();
  l.furniture = [
    { id: 'a', item: 'rug', level: 0, x: 6, z: 0, rot: 0 }, { id: 'b', item: 'desk', level: 0, x: 7, z: 1, rot: 0 },
    { id: 'c', item: 'rug', level: 0, x: 6, z: 1, rot: 0 },
  ];
  assert.deepEqual(errs(l), []);
});

test('title length counts runes, not utf-16 units', () => {
  const l = valid();
  l.title = rep('\u{1F600}', 60);
  assert.deepEqual(errs(l), []);
  l.title = rep('\u{1F600}', 61);
  assert.ok(errs(l).length > 0);
});

test('empty catalog refuses furniture', () => {
  assert.ok(validateLayout(valid(), []).errors.length > 0);
});
