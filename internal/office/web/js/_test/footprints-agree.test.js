// the js footprint table must match the go one (layout/data/footprints.json) number for number
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { FOOTPRINTS } from '../garden/footprints.js';

const goTable = JSON.parse(fs.readFileSync(fileURLToPath(new URL('../../../layout/data/footprints.json', import.meta.url)), 'utf8'));

test('js footprints equal footprints.json', () => {
  for (const [type, f] of Object.entries(goTable)) {
    assert.ok(FOOTPRINTS[type], 'js is missing ' + type);
    for (const k of Object.keys(f)) assert.equal(FOOTPRINTS[type][k], f[k], type + '.' + k);
  }
  assert.deepEqual(Object.keys(FOOTPRINTS).sort(), Object.keys(goTable).sort());
});
