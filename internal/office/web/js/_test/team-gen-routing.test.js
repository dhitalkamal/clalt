// routing on the layouts the go generators write: real desk and stool seats from the renderer
import test from 'node:test';
import assert from 'node:assert/strict';
import { planSeats, badgeCounts } from '../routing.js';
import { gardenSeats, gardenOverflow } from '../garden/seats.js';
import { fixture, TEAM } from './garden-fixtures.js';

const memberIds = (L) => L.pavilions.filter((p) => p.type === 'team').flatMap((p) => p.members);

for (const name of TEAM) {
  test(name + ': members take their desks, interns the stools, the rest shows as a badge', () => {
    const L = fixture(name);
    const seats = gardenSeats(L);
    const byId = new Map(seats.map((s) => [s.id, s]));
    const ids = memberIds(L);
    const roster = ids.map((id) => ({ key: 'team:' + id, kind: 'team', team: id, type: id, state: 'busy' }));
    const interns = [0, 1, 2, 3, 4].map((i) => ({ key: 's1/i' + i, kind: 'intern', team: ids[0], owner: 's1', account: 'a', type: ids[0], state: 'busy' }));
    const main = { key: 's1', kind: 'main', account: L.pavilions.find((p) => p.type === 'cabin').account, type: 'main', state: 'busy' };
    const plan = planSeats(L, [...roster, ...interns, main], null, null, { seats, overflow: gardenOverflow(L) });
    for (const id of ids) {
      const s = byId.get(plan.get('team:' + id).seat);
      assert.equal(s.kind, 'desk');
      assert.equal(s.type, 'team');
      assert.equal(s.member, id);
    }
    const stools = interns.map((i) => plan.get(i.key)).filter((e) => e && e.kind === 'seat');
    assert.equal(stools.length, 3);
    for (const e of stools) {
      const s = byId.get(e.seat);
      assert.equal(s.kind, 'stool');
      assert.equal(s.member, ids[0]);
    }
    assert.equal(badgeCounts(plan).get(ids[0]), 2);
    const used = [...plan.values()].filter((e) => e.kind === 'seat').map((e) => e.seat);
    assert.equal(new Set(used).size, used.length);
    assert.equal(byId.get(plan.get('s1').seat).type, 'cabin');
  });
}

// the page path: members and sessions through assignGardenSeats, as the renderer calls it
test('hidden interns get no slot and a badge, the others get slots', async () => {
  const { assignGardenSeats } = await import('../garden/slots.js');
  const { compileGarden } = await import('../garden/compat.js');
  const { updateMembers, classifySubs } = await import('../roster.js');
  const L = fixture('gen-team-rows-5');
  const ids = memberIds(L);
  const team = ids.map((id) => ({ id, name: id, desc: '', source: 'builtin', busy: 0, color: '#4f8edc' }));
  const acct = L.pavilions.find((p) => p.type === 'cabin').account;
  const s = { id: 's1', handle: 's1', role: 'main', account: acct, project: 'p', state: 'busy', leaving: false, subs: [] };
  for (let i = 0; i < 6; i++) s.subs.push({ id: 's1/a' + i, role: 'sub', type: ids[0], session: s, leaving: false, startedAt: i });
  classifySubs([s], new Set(ids));
  const members = updateMembers([], team, [s]);
  const { slots, badges } = assignGardenSeats(compileGarden(L), [s], null, null, members);
  assert.equal(badges.get(ids[0]), 2);
  assert.ok(slots.has('team:' + ids[0]));
  assert.equal([...slots.keys()].filter((k) => k.startsWith('s1/a')).length, 3);
  for (const sl of slots.values()) assert.ok(Number.isFinite(sl.x) && Number.isFinite(sl.z));
});
