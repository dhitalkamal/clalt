// which agents count as moving for the frame scheduler
import test from 'node:test';
import assert from 'node:assert/strict';
import { agentsMoving } from '../motion.js';

const ag = (mode, state, scale = 1) => ({ mode, scale, s: { state } });

test('no agents: nothing moves', () => assert.equal(agentsMoving([]), false));

test('seated idle and ended agents are a tiny animation, not motion', () => {
  assert.equal(agentsMoving([ag('seated', 'idle'), ag('seated', 'starting')]), false);
});

test('walking, leisure and standing agents move', () => {
  for (const mode of ['walking', 'leisure', 'standing']) assert.equal(agentsMoving([ag('seated', 'idle'), ag(mode, 'idle')]), true, mode);
});

test('a busy or needs-you agent animates keys and rings', () => {
  assert.equal(agentsMoving([ag('seated', 'busy')]), true);
  assert.equal(agentsMoving([ag('seated', 'needs-you')]), true);
});

test('an agent still growing or shrinking moves', () => {
  assert.equal(agentsMoving([ag('away', 'idle', 0.5)]), true);
});

test('an agent fully away and shrunk does not', () => assert.equal(agentsMoving([ag('away', 'ended', 0.001)]), false));
