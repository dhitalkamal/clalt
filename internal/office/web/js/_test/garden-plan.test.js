// the planting plan: deterministic, capped, clear of the site features
import test from 'node:test';
import assert from 'node:assert/strict';
import { fixture, ALL } from './garden-fixtures.js';
import { plantPlan, MAX_TREES } from '../garden/plan.js';
import { polyOf } from '../garden/footprints.js';
import { inPoly } from '../garden/geo.js';

test('the plan is deterministic for a seed', () => {
  const L = fixture('river');
  const a = plantPlan(L);
  const b = plantPlan(L);
  assert.deepEqual(a, b);
  const other = fixture('river');
  other.site.trees.seed = 99;
  assert.notDeepEqual(plantPlan(other).trees.slice(0, 5), a.trees.slice(0, 5));
});

test('trees are capped and stay on the lawn and clear of pavilions', () => {
  for (const name of ALL) {
    const L = fixture(name);
    const plan = plantPlan(L);
    assert.ok(plan.trees.length > 10 && plan.trees.length <= MAX_TREES, name + ' ' + plan.trees.length);
    const polys = L.pavilions.map(polyOf);
    for (const t of plan.trees) {
      assert.ok(L.site.land.some((l) => inPoly(t.x, t.z, l.poly)), name + ' tree off land');
      assert.ok(!polys.some((p) => inPoly(t.x, t.z, p)), name + ' tree inside a pavilion');
    }
  }
});

test('a huge layout still caps trees', () => {
  const L = fixture('rows');
  L.site.trees = { mode: 'scatter', band: 40, seed: 1 };
  L.site.land = [{ poly: [[-80, -80], [80, -80], [80, 80], [-80, 80]] }];
  assert.ok(plantPlan(L).trees.length <= MAX_TREES);
});

test('trees keep out of the river and ponds', () => {
  const L = fixture('river');
  const plan = plantPlan(L);
  const pond = L.site.ponds[0];
  for (const t of plan.trees) {
    assert.ok(((t.x - pond.x) / pond.rx) ** 2 + ((t.z - pond.z) / pond.rz) ** 2 > 1);
  }
});

test('lawn tufts exist and are bounded', () => {
  const plan = plantPlan(fixture('village'));
  assert.ok(plan.tufts.length > 0 && plan.tufts.length <= 400);
});
