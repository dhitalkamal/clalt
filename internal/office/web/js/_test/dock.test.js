import test from 'node:test';
import assert from 'node:assert/strict';
import { dockInsets } from '../dock.js';

const base = { vh: 900, gap: 12, controlsTop: 840, barTop: 0, barShown: false };

test('wide: the side panels stop a gap above the controls row', () => {
  assert.equal(dockInsets({ ...base, vw: 1440 }).side, 900 - 840 + 12);
});

test('wide: a wrapped (taller) controls row pushes the panels up', () => {
  assert.ok(dockInsets({ ...base, vw: 1440, controlsTop: 780 }).side > dockInsets({ ...base, vw: 1440 }).side);
});

test('narrow: the sheet clears the camera bar and the controls row above it', () => {
  const d = dockInsets({ ...base, vw: 600, controlsTop: 790, barTop: 842, barShown: true });
  assert.equal(d.sheet, 900 - 790 + 12);
});

test('narrow: a bar that sits higher than the controls decides', () => {
  assert.equal(dockInsets({ ...base, vw: 600, controlsTop: 860, barTop: 842, barShown: true }).sheet, 900 - 842 + 12);
});

test('narrow: without a bar the sheet still clears the controls', () => {
  assert.equal(dockInsets({ ...base, vw: 600, controlsTop: 840 }).sheet, 900 - 840 + 12);
});

test('insets are never negative', () => {
  const d = dockInsets({ ...base, vw: 1440, controlsTop: 2000 });
  assert.ok(d.side >= 0 && d.sheet >= 0);
});
