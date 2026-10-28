// the wall cutaway: which walls fade for a camera, how far, and the smoothing
import test from 'node:test';
import assert from 'node:assert/strict';
import { wallFacing, wallOpacity, stepFade, backdropColor } from '../nav/cutmath.js';

const RAD = Math.PI / 180;
// back wall at z = -6 facing out toward -z, left wall at x = -10 facing out toward -x
const back = { x: 0, z: -6, nx: 0, nz: -1 };
const left = { x: -10, z: 0, nx: -1, nz: 0 };

test('a camera behind a wall is on its outer side, one inside the room is not', () => {
  assert.ok(wallFacing({ x: 0, z: -20 }, back) > 0);
  assert.ok(wallFacing({ x: 0, z: 5 }, back) < 0);
  assert.ok(wallFacing({ x: -15, z: 0 }, left) > 0);
});

test('walls between the camera and the room fade at low tilt', () => {
  const cam = { x: -20, z: -20, el: 30 * RAD };
  assert.ok(wallOpacity({ cam, wall: back }) < 0.2);
  assert.ok(wallOpacity({ cam, wall: left }) < 0.2);
});

test('walls far from the camera stay solid', () => {
  const cam = { x: 5, z: 30, el: 30 * RAD };
  assert.equal(wallOpacity({ cam, wall: back }), 1);
  assert.equal(wallOpacity({ cam, wall: left }), 1);
});

test('a camera inside the hall fades nothing', () => {
  const cam = { x: 0, z: 0, el: 20 * RAD };
  assert.equal(wallOpacity({ cam, wall: back }), 1);
  assert.equal(wallOpacity({ cam, wall: left }), 1);
});

test('steep views keep the walls solid, the fade comes in between 50 and about 60 degrees', () => {
  const at = (deg) => wallOpacity({ cam: { x: 0, z: -20, el: deg * RAD }, wall: back });
  assert.equal(at(75), 1);
  assert.ok(at(40) < 0.2);
  assert.ok(at(55) > at(40) && at(55) < 1);
});

test('the fade is continuous in the camera position across the wall plane', () => {
  let prev = 1;
  for (let z = 2; z >= -12; z -= 0.25) {
    const o = wallOpacity({ cam: { x: 0, z, el: 20 * RAD }, wall: back });
    assert.ok(Math.abs(o - prev) < 0.25, `jump at z=${z}`);
    prev = o;
  }
});

test('stepFade eases toward the target, snaps when close, jumps under reduced motion', () => {
  const a = stepFade(1, 0.1, 0.016, false);
  assert.ok(a < 1 && a > 0.1);
  assert.equal(stepFade(0.1005, 0.1, 0.016, false), 0.1);
  assert.equal(stepFade(1, 0.1, 0.016, true), 0.1);
  let v = 1;
  for (let i = 0; i < 120; i++) v = stepFade(v, 0.1, 0.016, false);
  assert.equal(v, 0.1);
});

test('backdrop colour is a calm hex built from the hall colours', () => {
  const c = backdropColor('#bb9676', '#ffffff');
  assert.match(c, /^#[0-9a-f]{6}$/);
  assert.notEqual(c, backdropColor('#335577', '#ffffff'));
  assert.match(backdropColor(undefined, undefined), /^#[0-9a-f]{6}$/);
});
