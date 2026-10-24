// url hash, remembered pick and key map. pure, no dom.
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseHash, buildHash, pickTemplate, remember, recall, orderTemplates, CAMS } from '../viewstate.js';
import { actionFor } from '../keymap.js';

const TEMPLATES = [{ id: 'hall', title: 'Hall' }, { id: 'rows', title: 'Rows' }, { id: 'village', title: 'Village' }, { id: 'river', title: 'River' }, { id: 'street', title: 'Street' }];

test('parseHash reads template and cam', () => {
  assert.deepEqual(parseHash('#template=river&cam=hero'), { template: 'river', cam: 'hero' });
  assert.deepEqual(parseHash('template=rows'), { template: 'rows', cam: null });
  assert.deepEqual(parseHash(''), { template: null, cam: null });
});

test('parseHash drops values that are not safe ids', () => {
  assert.deepEqual(parseHash('#template=../etc&cam=nope'), { template: null, cam: null });
  assert.deepEqual(parseHash('#template=' + 'a'.repeat(41)), { template: null, cam: null });
  assert.deepEqual(parseHash('#template=River'), { template: null, cam: null });
});

test('buildHash round trips and omits empties', () => {
  assert.equal(buildHash({ template: 'river', cam: 'hero' }), '#template=river&cam=hero');
  assert.equal(buildHash({ template: 'river', cam: null }), '#template=river');
  assert.equal(buildHash({ template: null, cam: null }), '');
  assert.deepEqual(parseHash(buildHash({ template: 'street', cam: 'ground' })), { template: 'street', cam: 'ground' });
});

test('remember and recall use storage and survive a throwing one', () => {
  const mem = new Map();
  const ok = { getItem: (k) => mem.get(k) ?? null, setItem: (k, v) => mem.set(k, v) };
  remember(ok, 'river');
  assert.equal(recall(ok), 'river');
  const bad = { getItem() { throw new Error('blocked'); }, setItem() { throw new Error('blocked'); } };
  assert.doesNotThrow(() => remember(bad, 'river'));
  assert.equal(recall(bad), null);
  assert.equal(recall(null), null);
});

test('pickTemplate prefers the hash, then the remembered pick, then the server default', () => {
  assert.equal(pickTemplate({ hash: 'river', stored: 'rows', templates: TEMPLATES }), 'river');
  assert.equal(pickTemplate({ hash: null, stored: 'rows', templates: TEMPLATES }), 'rows');
  assert.equal(pickTemplate({ hash: 'nope', stored: 'rows', templates: TEMPLATES }), 'rows');
  assert.equal(pickTemplate({ hash: null, stored: null, templates: TEMPLATES }), null);
  // before the list is known any well formed id is trusted
  assert.equal(pickTemplate({ hash: 'river', stored: null, templates: null }), 'river');
});

test('orderTemplates puts hall first then rows village river street, always offers hall', () => {
  const shuffled = [TEMPLATES[3], TEMPLATES[1], TEMPLATES[4], TEMPLATES[2]];
  assert.deepEqual(orderTemplates(shuffled).map((t) => t.id), ['hall', 'rows', 'village', 'river', 'street']);
  assert.deepEqual(orderTemplates([{ id: 'custom', title: 'C' }, ...shuffled]).map((t) => t.id), ['hall', 'rows', 'village', 'river', 'street', 'custom']);
});

test('camera presets and their keys', () => {
  assert.deepEqual(CAMS.map((c) => c.id), ['top', 'plan', 'iso', 'hero', 'ground', 'sky']);
  assert.deepEqual(CAMS.map((c) => c.key), ['6', '7', '8', '9', 'g', '']);
});

const key = (k, extra = {}) => ({ key: k, ctrlKey: false, metaKey: false, altKey: false, target: { tagName: 'BODY' }, ...extra });

test('number keys pick templates in order', () => {
  const t = orderTemplates(TEMPLATES);
  assert.deepEqual(actionFor(key('1'), t), { type: 'template', id: 'hall' });
  assert.deepEqual(actionFor(key('4'), t), { type: 'template', id: 'river' });
  assert.equal(actionFor(key('6'), t).type, 'cam');
  assert.equal(actionFor(key('0'), t), null);
});

test('letter keys pick cameras, u spins, l toggles labels', () => {
  const t = orderTemplates(TEMPLATES);
  assert.deepEqual(actionFor(key('6'), t), { type: 'cam', id: 'top' });
  assert.deepEqual(actionFor(key('G'), t), { type: 'ground' });
  assert.deepEqual(actionFor(key('p'), t), { type: 'spin' });
  assert.deepEqual(actionFor(key('l'), t), { type: 'labels' });
});

test('keys are ignored with modifiers, in text fields and while editing', () => {
  const t = orderTemplates(TEMPLATES);
  assert.equal(actionFor(key('q', { ctrlKey: true }), t), null);
  assert.equal(actionFor(key('q', { metaKey: true }), t), null);
  assert.equal(actionFor(key('q', { target: { tagName: 'INPUT' } }), t), null);
  assert.equal(actionFor(key('q', { target: { tagName: 'DIV', isContentEditable: true } }), t), null);
  assert.equal(actionFor(key('q'), t, { editing: true }), null);
});
