import test from 'node:test';
import assert from 'node:assert/strict';
import { releaseSelection } from '../activity/release.js';

test('closing the drawer clears the pinned agent, region and hover card', () => {
  const calls = [];
  const app = { selected: { id: 1 }, selectedRegion: { id: 2 }, hover: null };
  releaseSelection(app, { showCard: (c) => calls.push(c) });
  assert.equal(app.selected, null);
  assert.equal(app.selectedRegion, null);
  assert.deepEqual(calls, [null]);
});

test('releaseSelection leaves a hovered agent alone', () => {
  const hover = { id: 3 };
  const app = { selected: { id: 1 }, selectedRegion: null, hover };
  releaseSelection(app, { showCard: () => {} });
  assert.equal(app.hover, hover);
});
