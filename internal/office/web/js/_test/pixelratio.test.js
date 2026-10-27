// pixel ratio choice: capped, never below 1, never NaN
import test from 'node:test';
import assert from 'node:assert/strict';
import { pickPixelRatio, DPR_CAP } from '../pixelratio.js';

test('the cap is 1.5', () => assert.equal(DPR_CAP, 1.5));

test('low density screens keep their ratio', () => {
  assert.equal(pickPixelRatio(1), 1);
  assert.equal(pickPixelRatio(1.25), 1.25);
});

test('retina and phones are capped', () => {
  assert.equal(pickPixelRatio(2), 1.5);
  assert.equal(pickPixelRatio(3), 1.5);
});

test('missing or broken values fall back to 1', () => {
  assert.equal(pickPixelRatio(undefined), 1);
  assert.equal(pickPixelRatio(0), 1);
  assert.equal(pickPixelRatio(NaN), 1);
  assert.equal(pickPixelRatio(0.5), 1);
});

test('a custom cap is honoured', () => assert.equal(pickPixelRatio(3, 2), 2));
