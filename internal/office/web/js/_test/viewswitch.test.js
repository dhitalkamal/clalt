import test from 'node:test';
import assert from 'node:assert/strict';
import { runSwitch } from '../viewswitch.js';

// records the order of every dep call
function rig(applyImpl) {
  const log = [];
  const state = { cam: 'ground', layout: null, edit: false };
  const deps = {
    resetNav: () => log.push('resetNav'),
    apply: async (l) => { log.push('apply'); if (applyImpl) await applyImpl(l); },
    notice: (m) => log.push('notice:' + m),
    keepClear: () => log.push('keepClear'),
    goCam: (id, instant) => log.push('goCam:' + id + ':' + instant),
    saveHash: () => log.push('saveHash:' + state.cam),
    closeEditor: () => log.push('closeEditor'),
    maybeEditor: () => log.push('maybeEditor'),
  };
  return { log, state, deps };
}

test('garden layout closes the editor', async () => {
  const { log, state, deps } = rig();
  await runSwitch(deps, state, { layout: { v: 2 }, edit: true });
  assert.ok(log.includes('closeEditor'));
});

test('hall layout leaves the editor alone', async () => {
  const { log, state, deps } = rig();
  await runSwitch(deps, state, { layout: { v: 1 }, edit: true });
  assert.ok(!log.includes('closeEditor'));
  assert.ok(log.includes('maybeEditor'));
});

test('failed apply still sends the camera and rethrows', async () => {
  const { log, state, deps } = rig(() => { throw new Error('boom'); });
  await assert.rejects(runSwitch(deps, state, { layout: { v: 2 }, edit: true }), /boom/);
  assert.ok(log.includes('goCam:sky:true'));
  assert.ok(log.indexOf('resetNav') < log.indexOf('apply'));
});

test('switch writes the new cam to the hash before the scene builds', async () => {
  const { log, state, deps } = rig();
  await runSwitch(deps, state, { layout: { v: 2 }, edit: false });
  assert.equal(state.cam, 'sky');
  assert.ok(log.includes('saveHash:sky'));
});
