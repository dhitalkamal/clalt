// team pavilion seats: one desk per member in members order, three intern stools beside each desk
import test from 'node:test';
import assert from 'node:assert/strict';
import { fixture } from './garden-fixtures.js';
import { gardenSeats } from '../garden/seats.js';
import { sizeOf, polyOf, deskCells } from '../garden/footprints.js';
import { inPoly } from '../garden/geo.js';

const NAMES = ['team-river', 'team-rows', 'team-village'];
const teamOf = (L) => L.pavilions.find((p) => p.type === 'team');
const kindOf = (seats, kind) => seats.filter((s) => s.type === 'team' && s.kind === kind);

test('team footprint follows the workspace rule', () => {
  const s = sizeOf({ type: 'team', cols: 4, rows: 1 });
  assert.ok(Math.abs(s.w - (2.1 * 4 + 0.2)) < 1e-9);
  assert.ok(Math.abs(s.d - (3.5 + 0.2)) < 1e-9);
});

test('one desk seat per member, in members order', () => {
  for (const name of NAMES) {
    const L = fixture(name);
    const p = teamOf(L);
    const desks = kindOf(gardenSeats(L), 'desk');
    assert.equal(desks.length, p.members.length, name);
    assert.deepEqual(desks.map((d) => d.member), p.members, name);
    const cells = deskCells(p);
    desks.forEach((d, i) => {
      assert.equal(d.col, cells[i].col);
      assert.equal(d.row, cells[i].row);
      assert.equal(d.pavilionId, p.id);
      assert.equal(d.account, null);
    });
  }
});

test('desk seats carry the same fields as workspace desks', () => {
  const L = fixture('team-river');
  const seats = gardenSeats(L);
  const ws = seats.find((s) => s.type === 'workspace' && s.kind === 'desk');
  const td = kindOf(seats, 'desk')[0];
  for (const k of Object.keys(ws)) assert.ok(k in td, k);
});

test('three stool seats per desk with index 0 to 2', () => {
  for (const name of NAMES) {
    const seats = gardenSeats(fixture(name));
    const desks = kindOf(seats, 'desk');
    const stools = kindOf(seats, 'stool');
    assert.equal(stools.length, desks.length * 3, name);
    for (const d of desks) {
      const mine = stools.filter((s) => s.member === d.member);
      assert.deepEqual(mine.map((s) => s.index).sort(), [0, 1, 2], d.member);
      for (const s of mine) {
        assert.equal(s.pavilionId, d.pavilionId);
        assert.equal(s.y, 0);
        assert.ok(Number.isFinite(s.x) && Number.isFinite(s.z) && Number.isFinite(s.rot));
        const dist = Math.hypot(s.x - d.x, s.z - d.z);
        assert.ok(dist > 0.6 && dist < 1.3, `${d.member} stool ${s.index} at ${dist}`);
      }
    }
  }
});

test('stools face their desk', () => {
  const seats = gardenSeats(fixture('team-river'));
  const d = kindOf(seats, 'desk')[0];
  for (const s of kindOf(seats, 'stool').filter((x) => x.member === d.member)) {
    const want = (Math.atan2(d.x - s.x, d.z - s.z) * 180) / Math.PI;
    const diff = Math.abs((((s.rot - want) % 360) + 540) % 360 - 180);
    assert.ok(diff < 1e-3, `rot ${s.rot} want ${want}`);
  }
});

test('desks and stools stay inside the pavilion footprint', () => {
  for (const name of NAMES) {
    const L = fixture(name);
    const poly = polyOf(teamOf(L));
    for (const s of gardenSeats(L).filter((x) => x.type === 'team')) assert.ok(inPoly(s.x, s.z, poly), `${name} ${s.id}`);
  }
});

test('seat ids are unique and stool ids hang off the desk id', () => {
  const seats = gardenSeats(fixture('team-river'));
  assert.equal(new Set(seats.map((s) => s.id)).size, seats.length);
  for (const s of kindOf(seats, 'stool')) assert.ok(s.id.includes(':stool-'), s.id);
});

test('fewer members than desks leave the extra desks without seats', () => {
  const L = fixture('team-rows');
  assert.equal(teamOf(L).members.length, 3);
  assert.equal(kindOf(gardenSeats(L), 'desk').length, 3);
});

test('a team pavilion without members has no seats and extra members are ignored', () => {
  const L = fixture('team-river');
  teamOf(L).members = undefined;
  assert.equal(gardenSeats(L).filter((s) => s.type === 'team').length, 0);
  const M = fixture('team-river');
  teamOf(M).members = Array.from({ length: 12 }, (_, i) => 'm' + i);
  assert.equal(kindOf(gardenSeats(M), 'desk').length, 8);
});

test('every other seat is unchanged by the team pavilion', () => {
  const base = gardenSeats(fixture('river'));
  const withTeam = gardenSeats(fixture('team-river')).filter((s) => s.type !== 'team');
  assert.deepEqual(withTeam, base);
});
