// a Copilot session gets a desk on every layout: the four garden templates and the hall
import test from 'node:test';
import assert from 'node:assert/strict';
import { planSeats } from '../routing.js';
import { gardenSeats, gardenOverflow } from '../garden/seats.js';
import { fixture, GEN } from './garden-fixtures.js';
import { hallLayout, hallCatalog } from './hall-fixture.js';

const COPILOT = '~copilot';
const mains = (accounts) => accounts.map((a, i) => ({ key: 'm' + i, kind: 'main', account: a, type: 'main', state: 'busy' }));
const cp = [{ key: 'cp-1', kind: 'main', account: COPILOT, type: 'main', state: 'busy' }, { key: 'cp-2', kind: 'main', account: COPILOT, type: 'main', state: 'needs-you' }];

for (const name of GEN.filter((n) => /-3$/.test(n))) {
  test(name + ': copilot mains sit at desks, claude seats do not move', () => {
    const L = fixture(name);
    const accounts = L.pavilions.filter((p) => p.type === 'cabin').map((p) => p.account);
    const garden = { seats: gardenSeats(L), overflow: gardenOverflow(L) };
    const base = planSeats(L, mains(accounts), null, null, garden);
    const plan = planSeats(L, [...mains(accounts), ...cp], null, null, garden);
    for (const k of base.keys()) assert.equal(plan.get(k).seat, base.get(k).seat);
    const seats = cp.map((c) => plan.get(c.key));
    for (const e of seats) assert.equal(e.kind, 'seat');
    assert.equal(new Set(seats.map((e) => e.seat)).size, 2);
  });

  test(name + ': a cabin tied to the copilot id is used by copilot sessions', () => {
    const L = fixture(name);
    L.pavilions.find((p) => p.type === 'cabin').account = COPILOT;
    const plan = planSeats(L, cp, null, null, { seats: gardenSeats(L), overflow: gardenOverflow(L) });
    const cabin = L.pavilions.find((p) => p.account === COPILOT).id;
    for (const c of cp) assert.equal(plan.get(c.key).room, cabin);
  });
}

test('the hall layout seats copilot mains too', () => {
  const L = hallLayout();
  const plan = planSeats(L, [...mains(['work', 'personal']), ...cp], hallCatalog());
  for (const c of cp) assert.equal(plan.get(c.key).kind, 'seat');
  const keys = [...plan.values()].filter((e) => e.kind === 'seat').map((e) => e.room + e.x + ',' + e.z);
  assert.equal(new Set(keys).size, keys.length);
});
