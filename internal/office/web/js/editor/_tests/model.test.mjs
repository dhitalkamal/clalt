import test from 'node:test';
import assert from 'node:assert/strict';
import * as m from '../model.js';
import { catalog, baseLayout } from './fixtures.mjs';

const cat = new Map(catalog.map((c) => [c.id, c]));

test('addRoom gives a fresh id, a default label and the level', () => {
  const l = baseLayout();
  const r = m.addRoom(l, { x: 12, z: 10, w: 4, d: 4 }, 'meeting', 0);
  assert.match(r.id, /^[a-z0-9-]{1,32}$/);
  assert.equal(l.rooms.filter((x) => x.id === r.id).length, 1);
  assert.equal(r.label, 'Meeting');
  assert.equal(r.level, 0);
  assert.equal(r.account, undefined);
});

test('moveRoom carries the furniture assigned to it', () => {
  const l = baseLayout();
  m.moveRoom(l, 'r2', 2, 3);
  assert.equal(l.rooms[1].x, 12);
  assert.equal(l.furniture[0].x, 13);
  assert.equal(l.furniture[0].z, 4);
});

test('removeRoom drops its furniture too', () => {
  const l = baseLayout();
  m.removeRoom(l, 'r2');
  assert.equal(l.rooms.length, 2);
  assert.equal(l.furniture.length, 0);
});

test('addFurniture assigns the room that holds its center', () => {
  const l = baseLayout();
  const f = m.addFurniture(l, cat, 'chair', 0, 14, 5, 0);
  assert.equal(f.room, 'r2');
  const g = m.addFurniture(l, cat, 'chair', 0, 30, 15, 0);
  assert.equal(g.room, undefined);
  assert.notEqual(f.id, g.id);
});

test('rotateFurniture steps rot and keeps the room', () => {
  const l = baseLayout();
  m.rotateFurniture(l, 'f1', cat);
  assert.equal(l.furniture[0].rot, 90);
  m.rotateFurniture(l, 'f1', cat);
  m.rotateFurniture(l, 'f1', cat);
  m.rotateFurniture(l, 'f1', cat);
  assert.equal(l.furniture[0].rot, 0);
});

test('setRoomType drops account unless cabins', () => {
  const l = baseLayout();
  l.rooms[1].account = 'work';
  m.updateRoom(l, 'r2', { type: 'meeting' });
  assert.equal(l.rooms[1].account, undefined);
});

test('setSize clamps to 8..120', () => {
  const l = baseLayout();
  m.setSize(l, 2, 500);
  assert.deepEqual(l.size, { w: 8, d: 120 });
});

test('rotateFurniture keeps the center as stable as the grid allows', () => {
  const l = baseLayout();
  // desk 3x2 at 11,1: center 12.5,2 -> rotated 2x3 anchors at 11.5->12 (round), 0.5->1 (round)
  m.rotateFurniture(l, 'f1', cat);
  const f = l.furniture[0];
  assert.equal(f.rot, 90);
  assert.deepEqual([f.x, f.z], [12, 1]);
  m.rotateFurniture(l, 'f1', cat);
  assert.deepEqual([f.x, f.z], [12, 2]);
});
