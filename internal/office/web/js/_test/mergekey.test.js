// static mesh merging: which meshes may share a draw call
import test from 'node:test';
import assert from 'node:assert/strict';
import { mergeKey, groupMergeable, mergeable } from '../garden/mergekey.js';

const mat = (uuid, extra = {}) => ({ uuid, ...extra });
const geo = (attrs = ['position', 'normal', 'uv'], indexed = true) => ({ attrs, indexed });
const mesh = (o = {}) => ({ isMesh: true, material: mat('m1'), geometry: geo(), castShadow: true, receiveShadow: true, userData: {}, renderOrder: 0, ...o });

test('same material and flags share a key', () => {
  assert.equal(mergeKey(mesh()), mergeKey(mesh()));
});

test('material, shadow flags, attributes, index and render order split keys', () => {
  const base = mergeKey(mesh());
  assert.notEqual(base, mergeKey(mesh({ material: mat('m2') })));
  assert.notEqual(base, mergeKey(mesh({ castShadow: false })));
  assert.notEqual(base, mergeKey(mesh({ receiveShadow: false })));
  assert.notEqual(base, mergeKey(mesh({ geometry: geo(['position', 'normal']) })));
  assert.notEqual(base, mergeKey(mesh({ geometry: geo(undefined, false) })));
  assert.notEqual(base, mergeKey(mesh({ renderOrder: 2 })));
});

test('attribute order does not matter', () => {
  assert.equal(mergeKey(mesh({ geometry: geo(['uv', 'position', 'normal']) })), mergeKey(mesh()));
});

test('meshes that must stay alone', () => {
  assert.equal(mergeable(mesh()), true);
  assert.equal(mergeable(mesh({ isInstancedMesh: true })), false);
  assert.equal(mergeable(mesh({ isSkinnedMesh: true })), false);
  assert.equal(mergeable(mesh({ material: [mat('a'), mat('b')] })), false);
  assert.equal(mergeable(mesh({ material: mat('t', { transparent: true }) })), false);
  assert.equal(mergeable(mesh({ userData: { itemId: 'x' } })), false);
  assert.equal(mergeable(mesh({ userData: { tick() {} } })), false);
  assert.equal(mergeable(mesh({ visible: false })), false);
  assert.equal(mergeable(mesh({ isMesh: false })), false);
});

test('big camera blockers stay alone so camera collision does not change', () => {
  assert.equal(mergeable(mesh(), { size: 2, top: 3 }), false);
  assert.equal(mergeable(mesh(), { size: 2, top: 0.5 }), true);
  assert.equal(mergeable(mesh(), { size: 0.3, top: 3 }), true);
});

test('groups keep one entry per key and drop single meshes', () => {
  const a = mesh(); const b = mesh(); const c = mesh({ material: mat('m2') }); const d = mesh({ material: mat('m3') });
  const groups = groupMergeable([a, b, c, d]);
  assert.equal(groups.length, 1);
  assert.deepEqual(groups[0].meshes, [a, b]);
});
