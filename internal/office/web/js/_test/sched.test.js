// frame scheduler: which rate the page draws at, and when the next frame is due
import test from 'node:test';
import assert from 'node:assert/strict';
import { createSched, HOLD_MS, IDLE_MS, INTERVAL } from '../sched.js';

test('starts in ambient and a first frame is due at once', () => {
  const s = createSched(0);
  assert.equal(s.mode(0), 'ambient');
  assert.equal(s.due(0), true);
});

test('agents moving gives the agents rate', () => {
  const s = createSched(0);
  s.setAgents(true);
  assert.equal(s.mode(1000), 'agents');
  s.ran(1000);
  assert.equal(s.due(1000 + INTERVAL.agents / 2), false);
  assert.equal(s.due(1000 + INTERVAL.agents), true);
});

test('nothing moving gives the ambient heartbeat', () => {
  const s = createSched(0);
  s.ran(1000);
  assert.equal(s.mode(1100), 'ambient');
  assert.equal(s.due(1000 + INTERVAL.ambient - 100), false);
  assert.equal(s.due(1000 + INTERVAL.ambient), true);
});

test('drag, wheel and key input run every frame, then fall back after the hold', () => {
  for (const kind of ['drag', 'wheel', 'key']) {
    const s = createSched(0);
    s.setAgents(true);
    s.ran(1000);
    s.input(1010, kind);
    assert.equal(s.mode(1020), 'active', kind);
    assert.equal(s.due(1020), true, kind);
    assert.equal(s.mode(1010 + HOLD_MS + 1), 'agents', kind);
  }
});

test('a held drag stays active however long it lasts', () => {
  const s = createSched(0);
  s.setDragging(true);
  assert.equal(s.mode(50000), 'active');
  s.setDragging(false);
  s.input(50000, 'drag');
  assert.equal(s.mode(50000 + HOLD_MS + 1), 'ambient');
});

test('camera movement keeps it active but is not user input', () => {
  const s = createSched(0);
  s.cameraMoved(IDLE_MS + 5000);
  assert.equal(s.mode(IDLE_MS + 5100), 'active');
  assert.equal(s.mode(IDLE_MS + 5000 + HOLD_MS + 1), 'idle');
});

test('plain pointer movement is at least the agents rate, not 60', () => {
  const s = createSched(0);
  s.ran(1000);
  s.input(1000, 'move');
  assert.equal(s.mode(1100), 'agents');
  assert.equal(s.mode(1000 + HOLD_MS + 1), 'ambient');
});

test('two minutes without input drops moving agents to the ambient rate', () => {
  const s = createSched(0);
  s.setAgents(true);
  assert.equal(s.mode(IDLE_MS - 1), 'agents');
  assert.equal(s.mode(IDLE_MS), 'idle');
  s.ran(IDLE_MS);
  assert.equal(s.due(IDLE_MS + INTERVAL.agents), false);
  assert.equal(s.due(IDLE_MS + INTERVAL.ambient), true);
});

test('any input wakes from idle at once and restarts the idle timer', () => {
  const s = createSched(0);
  s.setAgents(true);
  s.ran(IDLE_MS + 10);
  s.input(IDLE_MS + 20, 'move');
  assert.equal(s.due(IDLE_MS + 21), true);
  assert.equal(s.mode(IDLE_MS + 30), 'agents');
  s.ran(IDLE_MS + 30);
  assert.equal(s.mode(IDLE_MS + 20 + IDLE_MS - 1), 'agents');
});

test('wake forces the next frame without counting as input', () => {
  const s = createSched(0);
  s.ran(1000);
  assert.equal(s.due(1010), false);
  s.wake();
  assert.equal(s.due(1010), true);
  s.ran(1010);
  assert.equal(s.due(1020), false);
  assert.equal(s.mode(IDLE_MS), 'idle');
});

test('waitMs says how long until the next frame is due', () => {
  const s = createSched(0);
  s.ran(1000);
  assert.equal(s.waitMs(1000), INTERVAL.ambient);
  assert.equal(s.waitMs(1000 + INTERVAL.ambient + 5), 0);
  s.setDragging(true);
  assert.equal(s.waitMs(1001), 0);
});

test('a frame dt is capped by mode so slow frames keep time', () => {
  const s = createSched(0);
  s.setAgents(true);
  assert.equal(s.dtCap(10), 0.1);
  s.ran(0);
  assert.equal(s.dtCap(IDLE_MS + 1), 0.6);
});
