// the short line on an account card: where the sessions of the account are
import test from 'node:test';
import assert from 'node:assert/strict';
import { accountLine } from '../accountline.js';

const s = (state, away = false) => ({ state, away });

test('all busy', () => assert.equal(accountLine([s('busy'), s('busy'), s('busy')]), '3 working'));
test('a mix of working and on break', () => assert.equal(accountLine([s('busy'), s('idle', true), s('idle', true)]), '1 working, 2 on break'));
test('idle at the cabin is idle, not on break', () => assert.equal(accountLine([s('idle'), s('idle', true)]), '1 idle, 1 on break'));
test('needs you comes first', () => assert.equal(accountLine([s('needs-you'), s('busy')]), '1 needs you, 1 working'));
test('no sessions, no line', () => assert.equal(accountLine([]), ''));
