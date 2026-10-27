// the hover card of a room or pavilion: name, account and who is inside
import test from 'node:test';
import assert from 'node:assert/strict';
import { regionInfo } from '../regioncard.js';

const region = { kind: 'room', id: 'c1', type: 'cabin', label: 'Work', pavilion: 'c1', x0: 0, x1: 4, z0: 0, z1: 3, y: 0, level: 0 };
const layout = { v: 2, pavilions: [{ id: 'c1', type: 'cabin', account: 'work', label: 'Work' }, { id: 'w1', type: 'workspace' }] };
const accounts = new Map([['work', { label: 'Work account' }]]);
const who = (id, x, z, extra = {}) => ({ id, x, y: 0, z, label: id, ...extra });
const rowsOf = (info) => Object.fromEntries(info.rows);

test('a cabin card names the pavilion and its account and lists who is inside', () => {
  const info = regionInfo(region, layout, accounts, [who('amber', 1, 1), who('birch', 2, 2), who('far', 9, 9)]);
  assert.equal(info.title, 'Work');
  const rows = rowsOf(info);
  assert.equal(rows.account, 'Work account');
  assert.equal(rows['agents here'], '2');
  assert.equal(rows.inside, 'amber, birch');
});

test('an empty room says nobody is in', () => {
  const info = regionInfo(region, layout, accounts, []);
  assert.equal(rowsOf(info)['agents here'], '0');
  assert.equal(rowsOf(info).inside, undefined);
});

test('a pavilion without an account has no account row, and its type names it when it has no label', () => {
  const r = { ...region, id: 'w1', type: 'workspace', label: undefined, pavilion: 'w1' };
  const info = regionInfo(r, layout, accounts, []);
  assert.equal(info.title, 'Open workspace');
  assert.equal(rowsOf(info).account, undefined);
});

test('a treehouse side counts only agents on the deck', () => {
  const r = { ...region, id: 't:review', type: 'review', label: 'Review', pavilion: 't', y: 1.29, level: 1 };
  const info = regionInfo(r, layout, accounts, [who('up', 1, 1, { y: 1.29 }), who('down', 1, 1, { y: 0 })]);
  assert.equal(rowsOf(info)['agents here'], '1');
  assert.equal(rowsOf(info).inside, 'up');
});

test('a long list is cut with a count of the rest', () => {
  const crowd = Array.from({ length: 9 }, (_, i) => who('a' + i, 1, 1));
  const info = regionInfo(region, layout, accounts, crowd);
  assert.match(rowsOf(info).inside, /\+3 more$/);
});

test('agents that went away are not counted', () => {
  const info = regionInfo(region, layout, accounts, [who('gone', 1, 1, { away: true })]);
  assert.equal(rowsOf(info)['agents here'], '0');
});

test('regions of a compiled garden get readable pavilion names', async () => {
  const { fixture } = await import('./garden-fixtures.js');
  const { compileGarden } = await import('../garden/compat.js');
  const L = compileGarden(fixture('village'));
  const titles = L.regions.map((r) => regionInfo(r, L.layout, new Map(), []).title);
  assert.ok(titles.includes('Cafe') && titles.includes('Lobby') && titles.includes('Review'), titles.join(','));
});
