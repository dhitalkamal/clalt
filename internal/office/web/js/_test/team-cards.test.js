// hover card text for team members and interns, and the team pavilion region card
import test from 'node:test';
import assert from 'node:assert/strict';
import { teamCard } from '../cardinfo.js';
import { regionInfo } from '../regioncard.js';

const rows = (c) => Object.fromEntries(c.rows);

test('a member card shows name, description, busy count and the sessions using it', () => {
  const m = { role: 'team', name: 'Explore', desc: 'fast codebase search', busy: 2, state: 'busy', users: [{ handle: 'amber', project: 'clalt' }, { handle: 'birch', project: 'acorn' }] };
  const c = teamCard(m);
  assert.equal(c.title, 'Explore');
  const r = rows(c);
  assert.equal(r.about, 'fast codebase search');
  assert.equal(r.running, '2');
  assert.equal(r['used by'], 'amber (clalt), birch (acorn)');
});

test('an idle member says so and has no users row, empty description is dashed', () => {
  const c = teamCard({ role: 'team', name: 'Plan', desc: '', busy: 0, state: 'idle', users: [] });
  const r = rows(c);
  assert.equal(r.about, '-');
  assert.equal(r.running, '0');
  assert.equal(r['used by'], undefined);
  assert.notEqual(c.key, teamCard({ role: 'team', name: 'Plan', desc: '', busy: 1, state: 'busy', users: [] }).key);
});

test('an intern card shows its agent, its label and the project of the owner', () => {
  const owner = { id: 's1', handle: 's1', project: 'clalt' };
  const c = teamCard({ role: 'sub', intern: true, type: 'Explore', task: 'find the router', owner: 's1', session: owner, leaving: false }, 'Explore');
  assert.equal(c.title, 'Explore (intern)');
  const r = rows(c);
  assert.equal(r.task, 'find the router');
  assert.equal(r['works for'], 's1 (clalt)');
});

test('the team pavilion card counts members and who is working', () => {
  const region = { kind: 'room', id: 'tm', type: 'team', pavilion: 'tm', x0: 0, x1: 9, z0: 0, z1: 9, y: 0 };
  const info = regionInfo(region, { pavilions: [{ id: 'tm', type: 'team' }] }, null, [
    { id: 'team:a', label: 'Explore', x: 1, y: 0, z: 1, team: true, busy: true },
    { id: 'team:b', label: 'Plan', x: 2, y: 0, z: 2, team: true, busy: false },
    { id: 's1/x', label: 'Explore', x: 3, y: 0, z: 3 },
  ]);
  assert.equal(info.title, 'Team');
  const r = Object.fromEntries(info.rows);
  assert.equal(r.members, '2');
  assert.equal(r.working, '1');
});
