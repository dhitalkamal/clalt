// nav input collection: keys, pointers (pinch), wheel units
import test from 'node:test';
import assert from 'node:assert/strict';
import { createInput } from '../nav/input.js';

function setup() {
  const win = new EventTarget();
  const doc = new EventTarget();
  doc.visibilityState = 'visible';
  const dom = new EventTarget();
  dom.setPointerCapture = () => {};
  const old = [globalThis.window, globalThis.document, globalThis.innerHeight];
  globalThis.window = win;
  globalThis.document = doc;
  globalThis.innerHeight = 800;
  const input = createInput(dom);
  const fire = (t, type, props = {}) => { const e = new Event(type, { cancelable: true }); for (const [k, v] of Object.entries(props)) Object.defineProperty(e, k, { value: v }); t.dispatchEvent(e); return e; };
  const done = () => { input.dispose(); [globalThis.window, globalThis.document, globalThis.innerHeight] = old; };
  return { input, win, doc, dom, fire, done };
}

test('a keydown inside an open dialog does not walk', () => {
  const { input, win, fire, done } = setup();
  const inDialog = { tagName: 'BUTTON', closest: (q) => (q === 'dialog[open]' ? {} : null) };
  fire(win, 'keydown', { code: 'KeyW', target: { tagName: 'BODY', closest: () => null } });
  assert.ok(input.keys.has('KeyW'));
  fire(win, 'keydown', { code: 'KeyA', target: inDialog });
  assert.ok(!input.keys.has('KeyA'));
  fire(win, 'keydown', { code: 'KeyW', target: inDialog });
  assert.ok(!input.keys.has('KeyW'), 'held key dropped when focus moves into a dialog');
  done();
});

test('meta keydown and keyup clear held keys', () => {
  const { input, win, fire, done } = setup();
  fire(win, 'keydown', { code: 'KeyW', target: null });
  fire(win, 'keydown', { code: 'MetaLeft', key: 'Meta', metaKey: true, target: null });
  assert.equal(input.keys.size, 0);
  fire(win, 'keydown', { code: 'KeyW', target: null });
  fire(win, 'keyup', { code: 'MetaLeft', key: 'Meta', target: null });
  assert.equal(input.keys.size, 0);
  done();
});

test('hiding the page clears held keys', () => {
  const { input, win, doc, fire, done } = setup();
  fire(win, 'keydown', { code: 'KeyW', target: null });
  doc.visibilityState = 'hidden';
  fire(doc, 'visibilitychange');
  assert.equal(input.keys.size, 0);
  done();
});

test('wheel in line and page mode is normalised to pixels', () => {
  const { input, dom, fire, done } = setup();
  fire(dom, 'wheel', { deltaY: 3, deltaMode: 1, clientX: 0, clientY: 0 });
  assert.equal(input.wheel, 48);
  input.consume();
  fire(dom, 'wheel', { deltaY: 1, deltaMode: 2, clientX: 0, clientY: 0 });
  assert.equal(input.wheel, 800);
  done();
});

const ptr = (id, x, y, buttons = 1) => ({ pointerId: id, clientX: x, clientY: y, buttons, shiftKey: false, ctrlKey: false });

test('two touches pinch to wheel and pan, never orbit', () => {
  const { input, dom, fire, done } = setup();
  fire(dom, 'pointerdown', ptr(1, 100, 100));
  fire(dom, 'pointerdown', ptr(2, 200, 100));
  input.consume();
  fire(dom, 'pointermove', ptr(1, 50, 100));
  fire(dom, 'pointermove', ptr(2, 250, 100));
  assert.ok(input.wheel < 0, 'spreading zooms in: ' + input.wheel);
  assert.equal(input.ctrlWheel, true);
  assert.equal(input.mouse.dx, 0);
  assert.equal(input.mouse.dy, 0);
  input.consume();
  fire(dom, 'pointermove', ptr(1, 80, 130));
  fire(dom, 'pointermove', ptr(2, 280, 130));
  assert.equal(input.mouse.buttons, 2, 'pan through the right button path');
  assert.ok(input.mouse.dy > 0);
  done();
});

test('lifting a pinch finger does not jump the orbit', () => {
  const { input, dom, fire, done } = setup();
  fire(dom, 'pointerdown', ptr(1, 100, 100));
  fire(dom, 'pointerdown', ptr(2, 200, 100));
  fire(dom, 'pointerup', ptr(2, 200, 100, 0));
  input.consume();
  fire(dom, 'pointermove', ptr(1, 110, 100));
  assert.ok(Math.abs(input.mouse.dx) <= 10, 'dx ' + input.mouse.dx);
  assert.equal(input.mouse.buttons, 1);
  done();
});
