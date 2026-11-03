// applying a hall layout again must not leave the old roster layer (cabins, spare desks) in the world
import test from 'node:test';
import assert from 'node:assert/strict';
import { releaseRoster } from '../scene/release.js';

const fakeWorld = () => ({ children: [], add(g) { this.children.push(g); }, remove(g) { this.children = this.children.filter((c) => c !== g); } });

test('releasing the roster layer twice leaves one dynamic group in the world', () => {
  const world = fakeWorld();
  const drop = (g) => { if (g) world.remove(g); };
  const app = { agents: new Map([['a', {}]]), dynamic: null, dimmed: {} };
  const removed = [];
  const removeAgent = (id) => { removed.push(id); app.agents.delete(id); };
  for (let visit = 0; visit < 3; visit++) {
    releaseRoster(app, removeAgent, drop);
    // refreshSeats drops app.dynamic.group too, but releaseRoster has already cleared it
    drop(app.dynamic && app.dynamic.group);
    app.dynamic = { group: { visit } };
    world.add(app.dynamic.group);
  }
  assert.equal(world.children.length, 1);
  assert.deepEqual(removed, ['a']);
  assert.equal(app.dimmed, null);
});
