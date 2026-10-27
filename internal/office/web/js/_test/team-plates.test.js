// nameplate text mapping and the +N badge state
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { fixture } from './garden-fixtures.js';
import { gardenSeats } from '../garden/seats.js';
import { plateTexts, plateCell, PLATE_CELLS } from '../garden/teamtext.js';
import { syncBadges, setDeskBadge, badgeCount, badgeItems, resetBadges, onBadgeChange } from '../garden/badge.js';

const team = JSON.parse(fs.readFileSync(fileURLToPath(new URL('./fixtures/team-list.json', import.meta.url)), 'utf8'));

test('plate texts follow the members order', () => {
  const p = fixture('team-river').pavilions.find((x) => x.type === 'team');
  const t = plateTexts(p.members, team);
  assert.deepEqual(t.map((x) => x.id), p.members);
  assert.equal(t[0].name, 'General Purpose');
  assert.equal(t[1].name, 'Explore');
  assert.equal(t[1].color, '#e0a030');
});

test('a member missing from the team list falls back to its id', () => {
  const t = plateTexts(['explore', 'ghost'], team);
  assert.equal(t[1].name, 'ghost');
  assert.match(t[1].color, /^#[0-9a-f]{6}$/);
});

test('plate names are cut to 28 chars and stay ascii-safe strings', () => {
  const long = [{ id: 'x', name: 'N'.repeat(60), color: '#112233' }];
  assert.equal(plateTexts(['x'], long)[0].name.length, 28);
  assert.deepEqual(plateTexts([], team), []);
  assert.deepEqual(plateTexts(undefined, undefined), []);
});

test('plate cells are stable per index and stay inside the atlas', () => {
  const a = plateCell(0);
  const b = plateCell(5);
  assert.notDeepEqual(a, b);
  assert.deepEqual(plateCell(5), b);
  for (let i = 0; i < PLATE_CELLS; i++) {
    const c = plateCell(i);
    assert.ok(c.u0 >= 0 && c.u1 <= 1 && c.v0 >= 0 && c.v1 <= 1 && c.u1 > c.u0 && c.v1 > c.v0);
  }
});

test('badge counts: set, clamp, hide at 0', () => {
  resetBadges();
  assert.equal(badgeCount('explore'), 0);
  setDeskBadge('explore', 4);
  assert.equal(badgeCount('explore'), 4);
  setDeskBadge('explore', 0);
  assert.equal(badgeCount('explore'), 0);
  setDeskBadge('explore', -3);
  assert.equal(badgeCount('explore'), 0);
  setDeskBadge('explore', 2.7);
  assert.equal(badgeCount('explore'), 2);
  setDeskBadge('explore', 5000);
  assert.equal(badgeCount('explore'), 99);
});

test('badge items exist only for members with a count and sit over their desk', () => {
  resetBadges();
  const seats = gardenSeats(fixture('team-river'));
  assert.deepEqual(badgeItems(seats), []);
  setDeskBadge('plan', 3);
  const items = badgeItems(seats);
  assert.equal(items.length, 1);
  const d = seats.find((s) => s.kind === 'desk' && s.member === 'plan');
  assert.equal(items[0].cls, 'badge');
  assert.equal(items[0].key, '+3');
  assert.equal(items[0].x, d.x);
  assert.equal(items[0].z, d.z);
  assert.ok(items[0].y > 0.5);
  setDeskBadge('ghost', 2);
  assert.equal(badgeItems(seats).length, 1);
});

test('a badge change notifies the label layer once per change', () => {
  resetBadges();
  let n = 0;
  onBadgeChange(() => { n++; });
  setDeskBadge('plan', 2);
  setDeskBadge('plan', 2);
  setDeskBadge('plan', 0);
  assert.equal(n, 2);
  onBadgeChange(null);
});

test('syncBadges follows app.badges and clears members that left the map', () => {
  resetBadges();
  syncBadges(new Map([['explore', 2], ['plan', 1]]));
  assert.equal(badgeCount('explore'), 2);
  syncBadges(new Map([['explore', 5]]));
  assert.equal(badgeCount('explore'), 5);
  assert.equal(badgeCount('plan'), 0);
  syncBadges(undefined);
  assert.equal(badgeCount('explore'), 0);
});
